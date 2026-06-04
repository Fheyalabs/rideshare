//go:build openfhe

package auction

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
)

func auctionParams() cgo.ContractParams {
	return cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
}

func nonces(n int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = []byte(fmt.Sprintf("nonce-%d", i))
	}
	return out
}

var testWeights = Weights{K: 100, WStar: 1, WDist: 0.001}
var testBand = PriceBand{FloorCents: 800, CapCents: 5000}

// TestRunAuction_LowestPriceWins checks that the cheapest driver wins when prices differ.
// drv-A=€12.90, drv-B=€11.90 (cheapest), drv-C=€12.40. Expect drv-B wins and
// the revealed agreed price is within ±1 cent of 1190 (binding).
func TestRunAuction_LowestPriceWins(t *testing.T) {
	bids := []DriverBid{
		{Pseudonym: "drv-A", PriceCents: 1290, StarNorm: 4.8, DistSq: 1.0},
		{Pseudonym: "drv-B", PriceCents: 1190, StarNorm: 4.1, DistSq: 4.0},
		{Pseudonym: "drv-C", PriceCents: 1240, StarNorm: 4.5, DistSq: 2.5},
	}
	sessionID := []byte("ride-session-1")

	pkg, _, err := RunAuction(auctionParams(), bids, nonces(len(bids)), testWeights, testBand, 1, sessionID)
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	if pkg.Pseudonym != "drv-B" {
		t.Errorf("expected winner drv-B, got %q (index %d)", pkg.Pseudonym, pkg.WinnerIndex)
	}
	diff := pkg.AgreedPriceCents - 1190
	if diff < -1 || diff > 1 {
		t.Errorf("agreed price %d is not within ±1 of 1190 (binding check)", pkg.AgreedPriceCents)
	}
	t.Logf("winner=%s agreedCents=%d", pkg.Pseudonym, pkg.AgreedPriceCents)
}

// TestRunAuction_TieBrokenByStar verifies that when all prices are equal,
// the driver with the highest star rating wins.
func TestRunAuction_TieBrokenByStar(t *testing.T) {
	bids := []DriverBid{
		{Pseudonym: "drv-X", PriceCents: 1200, StarNorm: 4.9, DistSq: 2.0},
		{Pseudonym: "drv-Y", PriceCents: 1200, StarNorm: 4.0, DistSq: 1.0},
		{Pseudonym: "drv-Z", PriceCents: 1200, StarNorm: 4.2, DistSq: 3.0},
	}
	sessionID := []byte("ride-session-1")

	pkg, _, err := RunAuction(auctionParams(), bids, nonces(len(bids)), testWeights, testBand, 1, sessionID)
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	if pkg.Pseudonym != "drv-X" {
		t.Errorf("expected highest-★ winner drv-X (4.9★), got %q (index %d)", pkg.Pseudonym, pkg.WinnerIndex)
	}
	t.Logf("winner=%s starNorm=%.1f", pkg.Pseudonym, pkg.StarNorm)
}

// TestRunAuction_SharedSecret verifies the returned secret equals
// SHA256(nonce_winner || sessionID), proving the caller can reproduce it
// given the winner index.
func TestRunAuction_SharedSecret(t *testing.T) {
	bids := []DriverBid{
		{Pseudonym: "drv-A", PriceCents: 1290, StarNorm: 4.8, DistSq: 1.0},
		{Pseudonym: "drv-B", PriceCents: 1190, StarNorm: 4.1, DistSq: 4.0},
		{Pseudonym: "drv-C", PriceCents: 1240, StarNorm: 4.5, DistSq: 2.5},
	}
	sessionID := []byte("ride-session-1")
	ns := nonces(len(bids))

	pkg, secret, err := RunAuction(auctionParams(), bids, ns, testWeights, testBand, 1, sessionID)
	if err != nil {
		t.Fatalf("RunAuction: %v", err)
	}

	winnerIdx := pkg.WinnerIndex
	want := sha256.Sum256(append(append([]byte{}, ns[winnerIdx]...), sessionID...))
	if len(secret) != 32 {
		t.Fatalf("secret length %d, want 32", len(secret))
	}
	for i, b := range want {
		if secret[i] != b {
			t.Errorf("secret mismatch at byte %d: got %02x want %02x", i, secret[i], b)
		}
	}
	t.Logf("winner=%s secret=%x", pkg.Pseudonym, secret)
}
