package auction

import "testing"

func TestBuildKey_PriceStrictlyDominates(t *testing.T) {
	w := Weights{K: 100, WStar: 1.0, WDist: 0.001}
	// Cheaper price must win regardless of a worse ★ and farther distance.
	cheapBadStar := BuildKey(1200 /*€12.00*/, 4 /*1★ → penalty 4*/, 9.0, w)
	dearGoodStar := BuildKey(1201 /*€12.01*/, 0 /*5★ → penalty 0*/, 0.0, w)
	if !(cheapBadStar < dearGoodStar) {
		t.Fatalf("price must dominate: cheap=%.4f dear=%.4f", cheapBadStar, dearGoodStar)
	}
}

func TestBuildKey_TieBrokenByStarThenDistance(t *testing.T) {
	w := Weights{K: 100, WStar: 1.0, WDist: 0.001}
	// Equal price: higher ★ (lower penalty) wins.
	hiStar := BuildKey(1200, 0, 5.0, w)
	loStar := BuildKey(1200, 2, 0.0, w)
	if !(hiStar < loStar) {
		t.Fatalf("equal price → higher ★ wins: hi=%.4f lo=%.4f", hiStar, loStar)
	}
	// Equal price AND ★: nearer (smaller dist²) wins.
	near := BuildKey(1200, 1, 1.0, w)
	far := BuildKey(1200, 1, 9.0, w)
	if !(near < far) {
		t.Fatalf("equal price+★ → nearer wins: near=%.4f far=%.4f", near, far)
	}
}
