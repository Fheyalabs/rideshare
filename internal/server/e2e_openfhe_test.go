//go:build openfhe

package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/auction"
	"github.com/Fheyalabs/rideshare/internal/discovery"
)

// End-to-end real-FHE smoke: drivers push coarse grid cells, the rider opens a
// session from its own coarse cell (server-side hierarchical discovery + invites,
// no GPS), drivers fetch the rider pk + submit signed encrypted bids, the server
// runs the blind auction, the rider decrypts the masks and the cheapest wins.
func TestE2E_BlindAuction_DiscoveryToWinner(t *testing.T) {
	os.Setenv("ARES_FHE_ALLOW_INSECURE", "0")
	defer os.Setenv("ARES_FHE_ALLOW_INSECURE", "1")

	rs := NewRideshareServer(Config{})
	rs.RegisterOpenFHERoutes()
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	// 1. Rider keygen.
	cgoParams := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
	pk, sk, err := cgo.SingleKeyGen(cgoParams)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	// 2. Upload rider pk.
	pkHandle := putArtifact(t, ts.URL, pk)

	// 3. Drivers push their coarse grid cells (client-computed; server sees no GPS).
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
		cell := discovery.CellToString(discovery.CellAt(d.lat, d.lng, discovery.BaseResolution))
		pushGrid(t, ts.URL, d.name, cell, true)
		driverKeys[i], _ = sign.NewEd25519Signer()
		enc, _ := cgo.SingleKeyEncrypt(cgoParams, pk, float64(d.price))
		driverEncs[i] = enc
		driverNonces[i] = []byte(d.name)
	}

	// 4. Rider opens a session from its coarse cell (discovery + invites server-side).
	riderCell := discovery.CellToString(discovery.CellAt(51.0493, 13.7384, discovery.BaseResolution))
	dropoff := discovery.CellToString(discovery.CellAt(51.0600, 13.7500, discovery.BaseResolution-2))
	sessionID := "ride-e2e-001"
	cands := openSessionCell(t, ts.URL, sessionID, pkHandle, riderCell, 3, 4, 1500, dropoff)
	if len(cands) < 2 {
		t.Fatalf("expected >=2 candidates, got %d", len(cands))
	}

	// 4b. A candidate sees the invite (offered price + coarse dropoff, no exact loc).
	if invs := getInvites(t, ts.URL, cands[0]); len(invs) == 0 || invs[0].OfferedPrice != 1500 {
		t.Fatalf("candidate %s missing invite / offered price", cands[0])
	}

	// 5. Submit signed encrypted bids.
	for i := range drivers {
		bidHandle := putArtifact(t, ts.URL, driverEncs[i])
		sig, _ := auction.SignBid(driverKeys[i], []byte(sessionID), driverEncs[i], driverNonces[i])
		submitBid(t, ts.URL, sessionID, bidHandle, driverNonces[i],
			driverKeys[i].PublicKey(), sig)
	}

	// 6. Server runs the blind auction; rider fetches + decrypts masks.
	handles := getMasks(t, ts.URL, sessionID)
	var best int
	var bestVal float64
	for i, h := range handles {
		ct := getArtifact(t, ts.URL, h)
		vals, err := cgo.SingleKeyDecrypt(cgoParams, sk, ct, 1)
		if err != nil {
			t.Fatalf("decrypt mask[%d]: %v", i, err)
		}
		if i == 0 || vals[0] > bestVal {
			best, bestVal = i, vals[0]
		}
	}
	if best != 1 {
		t.Errorf("expected drv-B (€11.90) to win, got idx=%d", best)
	}
	t.Logf("E2E: winner=%d of %d masks", best, len(handles))
}

// --- openfhe-only HTTP helpers ---

func submitBid(t *testing.T, url, sessionID, bidHandle string, nonce, pubkey, sig []byte) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"session_id": sessionID, "bid_handle": bidHandle,
		"nonce": nonce, "pubkey": pubkey, "sig": sig,
	})
	resp, err := http.Post(url+"/session/bid", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("submit bid: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("submit bid: status %d", resp.StatusCode)
	}
}

func getMasks(t *testing.T, url, sessionID string) []string {
	t.Helper()
	resp, err := http.Get(url + "/session/" + sessionID + "/masks")
	if err != nil {
		t.Fatalf("get masks: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		MaskHandles []string `json:"mask_handles"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.MaskHandles
}
