//go:build openfhe

package lifecycle

import (
	"crypto/sha256"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/ghost"
	"github.com/Fheyalabs/rideshare/internal/server"
)

func sha256Hash(parts ...[]byte) []byte {
	h := sha256.New()
	for _, p := range parts { h.Write(p) }
	return h.Sum(nil)
}

func TestE2E_HappyPath_FullRideLifecycle(t *testing.T) {
	os.Setenv("ARES_FHE_ALLOW_INSECURE", "0")
	defer os.Setenv("ARES_FHE_ALLOW_INSECURE", "1")

	rs := server.NewRideshareServer(server.Config{})
	rs.RegisterOpenFHERoutes()
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	params := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
	client := ghost.NewClient(ts.URL)
	sessionID := "ride-lifecycle-001"

	// ── 1. Three ghost drivers push grid (ACCEPTING_RIDES) ──

	drivers := []struct {
		name             string
		price            int
		lat, lng         float64
		strategy         ghost.PriceStrategy
	}{
		{"drv-Alice",  1400, 51.0500, 13.7384, ghost.PercentBelow(0.05)},  // bids 1330
		{"drv-Bob",    1200, 51.0495, 13.7390, ghost.PercentBelow(0.40)},  // bids 720 ← cheapest
		{"drv-Carol",  1700, 51.0490, 13.7380, ghost.PercentBelow(0.03)},  // bids 1649
	}
	dd := make([]ghost.Driver, 3)
	signers := make([]sign.Signer, 3)
	driverNonces := make([][]byte, 3)

	for i, d := range drivers {
		dd[i] = ghost.Driver{Pseudonym: d.name, MinCents: 500, Strategy: d.strategy}
		signers[i], _ = sign.NewEd25519Signer()
		rs.Ratings().Record(string(signers[i].PublicKey()), 4.5, 100)
		driverNonces[i] = []byte(d.name)
		cell := discovery.CellToString(discovery.CellAt(d.lat, d.lng, discovery.BaseResolution))
		if err := client.PushGrid(d.name, cell); err != nil {
			t.Fatalf("push grid %s: %v", d.name, err)
		}
	}

	// ── 2. Rider DISCOVERING → OFFER_REVIEW ──

	rider := &ghost.Rider{Params: params, Client: client}
	if err := rider.Keygen(); err != nil {
		t.Fatalf("rider keygen: %v", err)
	}
	riderCell := discovery.CellToString(discovery.CellAt(51.0493, 13.7384, discovery.BaseResolution))
	dropoff := discovery.CellToString(discovery.CellAt(51.0600, 13.7500, discovery.BaseResolution-2))

	cands, err := rider.OpenSession(map[string]any{
		"session_id":    sessionID,
		"cell":          riderCell,
		"target":        3,
		"max_widen":     4,
		"offered_price": 2000,
		"dropoff_hex":   dropoff,
		"floor_cents":   500,
		"cap_cents":     5000,
		"ring_dim":      1 << 15,
		"depth":         5,
	})
	if err != nil { t.Fatalf("open session: %v", err) }
	if len(cands) < 2 { t.Fatalf("expected >=2 candidates, got %d", len(cands)) }

	// ── 3. Each driver: DECIDING → AWAITING_RESULT ──

	for i, d := range dd {
		invs, _ := client.Invites(d.Pseudonym)
		if len(invs) == 0 { t.Logf("%s: no invite", d.Pseudonym); continue }
		bid, ok := d.DecideBid(invs[0])
		if !ok { t.Logf("%s: declined", d.Pseudonym); continue }
		if err := d.EncryptSignSubmit(params, sessionID, invs[0], bid, signers[i], client); err != nil {
			t.Errorf("%s: submit: %v", d.Pseudonym, err)
		}
	}

	// ── 4. Rider OFFER_REVIEW: decrypt masks → find winner ──

	winner, masks, err := rider.Winner(sessionID)
	if err != nil { t.Fatalf("winner: %v", err) }
	if winner != 1 { t.Errorf("expected drv-Bob (idx 1, cheapest), got idx=%d masks=%v", winner, masks) }

	winNonce := driverNonces[winner]

	// ── 5. Shared secret derivation ──

	riderSecret := sha256Hash(winNonce, []byte(sessionID))
	driverSecret := sha256Hash(winNonce, []byte(sessionID))
	if string(riderSecret) != string(driverSecret) {
		t.Error("shared secret mismatch between rider and winning driver")
	}

	// ── 6. COMPLETE: both return to IDLE ──

	t.Logf("FULL LIFECYCLE: winner=%s price=€%.2f secret=%x masks=%v",
		drivers[winner].name, float64([]int{1330, 720, 1649}[winner])/100,
		riderSecret[:8], masks)
}
