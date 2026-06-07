package discovery

import "testing"

func TestRegistry_EscalatesUntilTarget(t *testing.T) {
	r := NewRegistry()
	base := CellAt(51.0493, 13.7384, BaseResolution)
	// One driver in a sibling base cell (shares the parent, not the base cell).
	r.SetCell("drv-1", CellAt(51.0500, 13.7390, BaseResolution))
	// Rider at base cell, wants >=1 candidate.
	cands, cell := r.Discover(base, 1, 3) // target=1, maxWiden=3
	if len(cands) < 1 {
		t.Fatalf("expected >=1 candidate after escalation, got %d (final cell %v)", len(cands), cell)
	}
}

func TestRegistry_EmptyReturnsNone(t *testing.T) {
	r := NewRegistry()
	cands, _ := r.Discover(CellAt(51.0493, 13.7384, BaseResolution), 1, 3)
	if len(cands) != 0 {
		t.Fatalf("expected 0 candidates, got %d", len(cands))
	}
}

func TestRegistry_RemoveAndReheartbeat(t *testing.T) {
	r := NewRegistry()
	c := CellAt(51.0493, 13.7384, BaseResolution)
	r.SetCell("drv-1", c)
	cands, _ := r.Discover(c, 1, 0)
	if len(cands) != 1 {
		t.Fatalf("expected 1, got %d", len(cands))
	}
	r.Remove("drv-1")
	cands, _ = r.Discover(c, 1, 0)
	if len(cands) != 0 {
		t.Fatalf("expected 0 after remove, got %d", len(cands))
	}
}
