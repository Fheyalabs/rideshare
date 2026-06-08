package ghost

import "testing"

func TestDriver_ThresholdAndPrice(t *testing.T) {
	d := Driver{Pseudonym: "drv-1", MinCents: 1000, Strategy: PercentBelow(0.10)}
	// offer above threshold → bids 10% below
	bid, ok := d.DecideBid(Invite{OfferedPrice: 2000})
	if !ok || bid != 1800 {
		t.Fatalf("bid=%d ok=%v, want 1800 true", bid, ok)
	}
	// offer below threshold → declines
	if _, ok := d.DecideBid(Invite{OfferedPrice: 800}); ok {
		t.Error("offer below min-€ threshold must be declined")
	}
}

func TestDriver_FixedPrice(t *testing.T) {
	d := Driver{Pseudonym: "drv-2", MinCents: 500, Strategy: Fixed(1200)}
	bid, ok := d.DecideBid(Invite{OfferedPrice: 1500})
	if !ok || bid != 1200 {
		t.Fatalf("fixed bid=%d ok=%v, want 1200 true", bid, ok)
	}
}
