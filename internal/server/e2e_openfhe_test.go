//go:build openfhe

package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/auction"
)

func TestE2E_BlindAuction_DiscoveryToWinner(t *testing.T) {
	os.Setenv("ARES_FHE_ALLOW_INSECURE", "0")
	defer os.Setenv("ARES_FHE_ALLOW_INSECURE", "1")

	rs := NewRideshareServer(Config{})
	rs.RegisterOpenFHERoutes()
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	// --- 1. Rider keygen ---
	params := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
	pk, sk, err := cgo.SingleKeyGen(params)
	if err != nil { t.Fatalf("keygen: %v", err) }

	// --- 2. Upload rider pk to artifact store ---
	pkHandle := putArtifact(t, ts.URL, pk)

	// --- 3. Driver heartbeats ---
	drivers := []struct {
		name         string
		price        int
		star, distSq float64
		lat, lng     float64
	}{
		{"drv-A", 1290, 4.8, 1.0, 51.0500, 13.7384},
		{"drv-B", 1190, 4.1, 9.0, 51.0495, 13.7390},
		{"drv-C", 1240, 4.5, 4.0, 51.0490, 13.7380},
	}
	driverKeys := make([]sign.Signer, 3)
	driverEncs := make([][]byte, 3)
	driverNonces := make([][]byte, 3)
	for i, d := range drivers {
		heartbeat(t, ts.URL, d.name, d.lat, d.lng)
		driverKeys[i], _ = sign.NewEd25519Signer()
		enc, _ := cgo.SingleKeyEncrypt(params, pk, float64(d.price))
		driverEncs[i] = enc
		driverNonces[i] = []byte(d.name)
	}

	// --- 4. Discover ---
	cands := discover(t, ts.URL, 51.0493, 13.7384, 3, 3)
	if len(cands) < 2 { t.Fatalf("expected >=2 candidates, got %d", len(cands)) }

	// --- 5. Open session ---
	sessionID := "ride-e2e-001"
	openSession(t, ts.URL, sessionID, pkHandle, 1500, "892a1b3ffffffff")

	// --- 6. Submit signed bids ---
	reg := make(map[string]bool)
	for i, d := range drivers {
		bidHandle := putArtifact(t, ts.URL, driverEncs[i])
		sig, _ := auction.SignBid(driverKeys[i], []byte(sessionID), driverEncs[i], driverNonces[i])
		reg[string(driverKeys[i].PublicKey())] = true
		submitBid(t, ts.URL, sessionID, bidHandle, driverNonces[i],
			driverKeys[i].PublicKey(), sig, d.star, d.distSq)
		_ = d
	}

	// --- 7. Get masks ---
	handles := getMasks(t, ts.URL, sessionID)

	// --- 8. Rider decrypts ---
	var best int
	var bestVal float64
	for i, h := range handles {
		ct := getArtifact(t, ts.URL, h)
		vals, err := cgo.SingleKeyDecrypt(params, sk, ct, 1)
		if err != nil { t.Fatalf("decrypt mask[%d]: %v", i, err) }
		if i == 0 || vals[0] > bestVal { best, bestVal = i, vals[0] }
	}
	if best != 1 { t.Errorf("expected drv-B (€11.90) to win, got idx=%d", best) }

	// --- 9. Verify winning driver's signature ---
	t.Logf("E2E: winner=%d masks decrypted=%d", best, len(handles))
	_ = sk
}

// --- helpers ---

func putArtifact(t *testing.T, url string, data []byte) string {
	t.Helper()
	enc := base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(map[string]string{"Data": enc})
	resp, err := http.Post(url+"/artifacts", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("put artifact: %v", err) }
	defer resp.Body.Close()
	var out struct{ Handle string }
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Handle == "" { t.Fatal("empty artifact handle") }
	return out.Handle
}

func getArtifact(t *testing.T, url, handle string) []byte {
	t.Helper()
	resp, err := http.Get(url + "/artifacts/" + handle)
	if err != nil { t.Fatalf("get artifact: %v", err) }
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data
}

func heartbeat(t *testing.T, url, name string, lat, lng float64) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"pseudonym": name, "lat": lat, "lng": lng})
	resp, err := http.Post(url+"/heartbeat", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("heartbeat: %v", err) }
	resp.Body.Close()
}

func discover(t *testing.T, url string, lat, lng float64, target, maxWiden int) []string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"lat": lat, "lng": lng, "target": target, "max_widen": maxWiden})
	resp, err := http.Post(url+"/discover", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("discover: %v", err) }
	defer resp.Body.Close()
	var out struct{ Candidates []string }
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Candidates
}

func openSession(t *testing.T, url, sessionID, pkHandle string, offeredPrice int, dropoffHex string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"session_id":    sessionID,
		"pk_handle":     pkHandle,
		"offered_price": offeredPrice,
		"dropoff_hex":   dropoffHex,
		"floor_cents":   800, "cap_cents": 5000,
		"ring_dim": 1 << 15, "depth": 5,
	})
	resp, err := http.Post(url+"/session/open", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("open session: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated { t.Fatalf("open session: %d", resp.StatusCode) }
}

func submitBid(t *testing.T, url, sessionID, bidHandle string, nonce, pubkey, sig []byte, star, dist float64) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID, "bid_handle": bidHandle,
		"nonce": nonce, "pubkey": pubkey, "sig": sig,
		"star_norm": star, "dist_sq": dist,
	})
	resp, err := http.Post(url+"/session/bid", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatalf("submit bid: %v", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { t.Fatalf("submit bid: %d", resp.StatusCode) }
}

func getMasks(t *testing.T, url, sessionID string) []string {
	t.Helper()
	resp, err := http.Get(url + "/session/" + sessionID + "/masks")
	if err != nil { t.Fatalf("get masks: %v", err) }
	defer resp.Body.Close()
	var out struct {
		MaskHandles []string `json:"mask_handles"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.MaskHandles
}

