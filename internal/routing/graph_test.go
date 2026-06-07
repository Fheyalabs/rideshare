package routing

import (
	"math"
	"testing"
)

func TestGraphBuildAndWeight(t *testing.T) {
	g := NewGraph()
	g.AddNode(1, 51.0493, 13.7384)
	g.AddNode(2, 51.0593, 13.7384) // ~1.11 km due north
	g.AddEdge(1, 2, ClassPrimary, 50, false)

	// haversine ≈ 1112 m (0.01° lat). Allow 1% tolerance.
	e := g.Edges[0]
	if math.Abs(e.LenM-1112) > 12 {
		t.Errorf("edge length = %.1f m, want ≈1112", e.LenM)
	}
	// base weight (s) = LenM / (50 km/h in m/s) ≈ 1112 / 13.888 ≈ 80.1 s
	if w := g.BaseWeightSec(0); math.Abs(w-80.1) > 1 {
		t.Errorf("base weight = %.1f s, want ≈80.1", w)
	}
	if len(g.Adj[1]) != 1 || g.Adj[1][0] != 0 {
		t.Errorf("adjacency not wired: %v", g.Adj[1])
	}
	// oneway=false ⇒ reverse edge too
	if len(g.Adj[2]) != 1 {
		t.Errorf("expected reverse edge for two-way road")
	}
}
