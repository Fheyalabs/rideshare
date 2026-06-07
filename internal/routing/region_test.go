package routing

import "testing"

func TestSerializeRoundTripAndSlice(t *testing.T) {
	g := NewGraph()
	g.AddNode(1, 51.05, 13.74)
	g.AddNode(2, 51.06, 13.74)
	g.AddNode(3, 52.00, 14.50) // far away
	g.AddEdge(1, 2, ClassPrimary, 50, false)
	g.AddEdge(2, 3, ClassPrimary, 50, false)

	b, err := Serialize(g)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	g2, err := Deserialize(b)
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(g2.Edges) != len(g.Edges) || len(g2.Nodes) != len(g.Nodes) {
		t.Fatalf("round-trip mismatch")
	}

	// slice to nodes {1,2}: keeps only the 1<->2 edges, drops 2->3 / 3->2.
	s := Slice(g, map[int64]bool{1: true, 2: true})
	for _, e := range s.Edges {
		if e.From == 3 || e.To == 3 {
			t.Fatalf("slice leaked node 3")
		}
	}
	if len(s.Edges) != 2 { // 1->2 and 2->1
		t.Fatalf("want 2 sliced edges, got %d", len(s.Edges))
	}
}
