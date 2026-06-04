package rating

import (
	"math"
	"testing"
)

func TestNormalize_ShrinksLowCountTowardGlobalMean(t *testing.T) {
	const global, confidence = 4.0, 50.0
	// 4.5 over 50 ratings vs 4.3 over 300 ratings: shrinkage pulls the
	// low-count 4.5 down toward 4.0, the high-count 4.3 barely moves —
	// they should land within 0.1 of each other.
	hi := Normalize(4.5, 50, global, confidence)   // ≈ 4.25
	lo := Normalize(4.3, 300, global, confidence)   // ≈ 4.257
	if math.Abs(hi-lo) > 0.1 {
		t.Fatalf("expected close after shrinkage: hi=%.3f lo=%.3f", hi, lo)
	}
}

func TestNormalize_HighCountApproachesRaw(t *testing.T) {
	got := Normalize(4.8, 100000, 4.0, 50.0)
	if math.Abs(got-4.8) > 0.01 {
		t.Fatalf("high count should approach raw mean: got %.3f", got)
	}
}
