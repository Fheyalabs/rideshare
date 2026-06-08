package movement

import (
	"testing"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

func TestActor_AdvanceAndCell(t *testing.T) {
	g := routing.NewGraph()
	g.AddNode(1, 51.0500, 13.7400)
	g.AddNode(2, 51.0600, 13.7400) // ~1.1 km north
	g.AddEdge(1, 2, routing.ClassPrimary, 50, true)

	a := NewActor("drv-1", g, []int64{1, 2})
	lat0, lon0 := a.LatLng()
	if lat0 != 51.0500 {
		t.Fatalf("actor should start at node 1, got lat %.4f", lat0)
	}
	_ = lon0
	// advance ~100 s at ~13.9 m/s ≈ 1.39 km > edge length (~1.11 km) ⇒ reaches node 2
	a.Advance(100, 13.9)
	lat1, _ := a.LatLng()
	if lat1 < 51.055 {
		t.Errorf("after advancing, actor should be near node 2, got lat %.4f", lat1)
	}
	if a.Cell() == 0 {
		t.Error("Cell() should be a valid H3 cell")
	}
	if !a.Arrived() {
		t.Error("actor should have arrived at the track end")
	}
}
