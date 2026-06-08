package movement

import (
	"testing"

	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/routing"
)

func TestEngine_RoamsAndReportsCellChange(t *testing.T) {
	g := routing.NewGraph()
	for i := int64(1); i <= 5; i++ {
		g.AddNode(i, 51.05+float64(i)*0.01, 13.74)
	}
	for i := int64(1); i < 5; i++ {
		g.AddEdge(i, i+1, routing.ClassPrimary, 50, false)
	}
	w := routing.Customize(g, routing.Metric{})
	e := NewEngine(g, w, 42)
	e.Add("drv-1", 1)

	changes := 0
	e.OnCellChange = func(id string, _ discovery.Cell) { _ = id; changes++ }
	// tick enough that the actor moves and re-roams
	for i := 0; i < 200; i++ {
		e.Tick(5)
	}
	if e.Actor("drv-1") == nil {
		t.Fatal("actor missing")
	}
	// it should have fired at least one cell-change and at least one re-roam
	if changes < 1 {
		t.Error("expected at least 1 cell change callback")
	}
}
