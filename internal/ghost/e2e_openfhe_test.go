//go:build openfhe

package ghost

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/server"
)

func TestE2E_GhostFleetLoop(t *testing.T) {
	os.Setenv("ARES_FHE_ALLOW_INSECURE", "0")
	defer os.Setenv("ARES_FHE_ALLOW_INSECURE", "1")

	rs := server.NewRideshareServer(server.Config{})
	rs.RegisterOpenFHERoutes()
	ts := httptest.NewServer(rs.Handler())
	defer ts.Close()

	params := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}

	// 3 ghost drivers at nearby cells. drv-B is cheapest → should win.
	drivers := []struct {
		name     string
		price    int
		lat, lng float64
		strategy PriceStrategy
	}{
		{"drv-A", 1400, 51.0500, 13.7384, PercentBelow(0.05)},
		{"drv-B", 1200, 51.0495, 13.7390, PercentBelow(0.40)}, // bids 1200 ← cheapest
		{"drv-C", 1700, 51.0490, 13.7380, PercentBelow(0.05)},
	}
	dd := make([]Driver, 3)
	signers := make([]sign.Signer, 3)
	for i, d := range drivers {
		dd[i] = Driver{Pseudonym: d.name, MinCents: 800, Strategy: d.strategy}
		signers[i], _ = sign.NewEd25519Signer()
		cell := discovery.CellToString(discovery.CellAt(d.lat, d.lng, discovery.BaseResolution))
		NewClient(ts.URL).PushGrid(d.name, cell)
		// Seed rating store by pubkey so ★ is consistent
		rs.Ratings().Record(string(signers[i].PublicKey()), 4.5, 100)
	}

	// Rider
	rider := &Rider{Params: params, Client: NewClient(ts.URL)}
	if err := rider.Keygen(); err != nil {
		t.Fatalf("rider keygen: %v", err)
	}
	riderCell := discovery.CellToString(discovery.CellAt(51.0493, 13.7384, discovery.BaseResolution))
	dropoff := discovery.CellToString(discovery.CellAt(51.0600, 13.7500, discovery.BaseResolution-2))
	sessionID := "ghost-e2e-001"

	cands, err := rider.OpenSession(map[string]any{
		"session_id":    sessionID,
		"cell":          riderCell,
		"target":        3,
		"max_widen":     4,
		"offered_price": 2000,
		"dropoff_hex":   dropoff,
		"floor_cents":   800,
		"cap_cents":     5000,
		"ring_dim":      1 << 15,
		"depth":         5,
	})
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	if len(cands) < 2 {
		t.Fatalf("expected >=2 candidates, got %d", len(cands))
	}

	// Each ghost submits a real-crypto bid
	ghostClient := NewClient(ts.URL)
	for i, d := range dd {
		invs, _ := ghostClient.Invites(d.Pseudonym)
		if len(invs) == 0 {
			t.Logf("%s: no invite (may not be in discovery range)", d.Pseudonym)
			continue
		}
		bid, ok := d.DecideBid(invs[0])
		if !ok {
			t.Logf("%s: declined invite (offer below min-€)", d.Pseudonym)
			continue
		}
		if err := d.EncryptSignSubmit(params, sessionID, invs[0], bid, signers[i], ghostClient); err != nil {
			t.Errorf("%s: submit: %v", d.Pseudonym, err)
		}
	}

	// Rider decrypts masks → cheapest ghost wins
	winner, masks, err := rider.Winner(sessionID)
	if err != nil {
		t.Fatalf("winner: %v", err)
	}
	// drv-B bids 1200, cheapest
	if winner != 1 {
		t.Errorf("expected winner drv-B (idx 1, €12.00), got idx=%d masks=%v", winner, masks)
	}
	t.Logf("GHOST FLEET E2E: winner=%d masks=%v", winner, masks)
}
