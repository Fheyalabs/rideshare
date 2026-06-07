package rating

import "testing"

func TestStore_BayesianStar(t *testing.T) {
	s := NewStore(4.3, 20) // global mean 4.3, confidence 20
	s.Record("drv-A", 4.9, 5)   // few votes → shrunk toward global
	s.Record("drv-B", 4.4, 500) // many votes → near its own mean
	a, b := s.StarNorm("drv-A"), s.StarNorm("drv-B")
	if !(a > 4.3 && a < 4.9) {
		t.Errorf("A shrunk star = %.3f, want in (4.3,4.9)", a)
	}
	if b < 4.39 || b > 4.41 {
		t.Errorf("B star = %.3f, want ≈4.4", b)
	}
	if s.StarNorm("unknown") != 4.3 {
		t.Errorf("unknown driver = %.3f, want global mean 4.3", s.StarNorm("unknown"))
	}
}
