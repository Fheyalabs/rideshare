package routing

import "testing"

func TestAStar_ShortestPath(t *testing.T) {
	g := NewGraph()
	g.AddNode(1, 51.050, 13.740)
	g.AddNode(2, 51.055, 13.740)
	g.AddNode(3, 51.060, 13.740)
	g.AddNode(4, 51.060, 13.745)
	g.AddEdge(1, 2, ClassPrimary, 50, false)
	g.AddEdge(2, 3, ClassPrimary, 50, false)
	g.AddEdge(3, 4, ClassMotorway, 120, false)
	w := Customize(g, Metric{})

	path, distM, durS, ok := AStar(g, 1, 4, w)
	if !ok {
		t.Fatal("expected a path 1→4")
	}
	if len(path) != 4 || path[0] != 1 || path[3] != 4 {
		t.Fatalf("path = %v, want [1 2 3 4]", path)
	}
	if distM <= 0 || durS <= 0 {
		t.Fatalf("dist=%.1f dur=%.1f must be positive", distM, durS)
	}
	// two-way edges → 4→1 reachable
	if _, _, _, ok := AStar(g, 4, 1, w); !ok {
		t.Error("4→1 should be reachable (two-way edges)")
	}
}

func TestAStar_Unreachable(t *testing.T) {
	g := NewGraph()
	g.AddNode(1, 51.05, 13.74)
	g.AddNode(2, 51.06, 13.74)
	// no edge → unreachable
	_, _, _, ok := AStar(g, 1, 2, Customize(g, Metric{}))
	if ok {
		t.Error("unreachable must return ok=false")
	}
}
