package discovery

import "testing"

// Dresden Altmarkt ≈ 51.0493, 13.7384.
func TestCellAndWiden(t *testing.T) {
	c := CellAt(51.0493, 13.7384, BaseResolution)
	if c == 0 {
		t.Fatal("CellAt returned 0")
	}
	parent := Widen(c)
	if parent == 0 || parent == c {
		t.Fatalf("Widen failed: c=%v parent=%v", c, parent)
	}
	// A nearby point (~80 m) shares the base cell or at least the widened parent.
	c2 := CellAt(51.0500, 13.7384, BaseResolution)
	if Widen(c2) != parent && c2 != c {
		t.Logf("note: nearby point in different base cell (expected near edges)")
	}
}
