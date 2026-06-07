package routing

import "testing"

func TestBuildGraph(t *testing.T) {
	coords := map[int64][2]float64{
		10: {51.0493, 13.7384}, 11: {51.0593, 13.7384}, 12: {51.0593, 13.7484},
	}
	ways := []WayRec{
		{Nodes: []int64{10, 11}, Highway: "primary", Oneway: false, Ref: ""},
		{Nodes: []int64{11, 12}, Highway: "motorway", Oneway: true, Ref: "A17", MaxSpeedKmh: 120},
		{Nodes: []int64{10, 12}, Highway: "footway"}, // dropped (not drivable)
	}
	g := BuildGraph(ways, coords)
	// primary two-way (2 dir edges) + motorway oneway (1) = 3 edges; footway dropped.
	if len(g.Edges) != 3 {
		t.Fatalf("want 3 edges, got %d", len(g.Edges))
	}
	// the motorway edge carries Ref + class
	var found bool
	for _, e := range g.Edges {
		if e.Class == ClassMotorway {
			found = true
			if e.Ref != "A17" || e.MaxSpeedKmh != 120 {
				t.Errorf("motorway edge missing ref/speed: %+v", e)
			}
		}
	}
	if !found {
		t.Error("motorway edge missing")
	}
}
