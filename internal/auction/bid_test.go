package auction

import "testing"

func TestSelectCircuitSize_ClampsToFamily(t *testing.T) {
	if got := SelectCircuitSize(0); got != 1 {
		t.Fatalf("min size is 1, got %d", got)
	}
	if got := SelectCircuitSize(3); got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
	if got := SelectCircuitSize(9); got != MaxDrivers {
		t.Fatalf("ceiling is %d, got %d", MaxDrivers, got)
	}
}

func TestDriverBid_OfferPenalty(t *testing.T) {
	b := DriverBid{Pseudonym: "drv-1", PriceCents: 1250, StarNorm: 4.5, DistSq: 4.0}
	if p := b.StarPenalty(); p != 0.5 {
		t.Fatalf("5-★ penalty: want 0.5, got %.3f", p)
	}
}
