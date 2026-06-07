package routing

import (
	"encoding/json"
	"math"
	"testing"
)

func TestCustomize(t *testing.T) {
	g := NewGraph()
	g.AddNode(1, 51.05, 13.74)
	g.AddNode(2, 51.06, 13.74)
	g.AddEdge(1, 2, ClassPrimary, 50, true) // one edge, idx 0
	base := g.BaseWeightSec(0)

	// jam: primary at half speed ⇒ double the time
	w := Customize(g, Metric{ClassMult: map[RoadClass]float64{ClassPrimary: 0.5}})
	if math.Abs(w[0]-2*base) > 0.5 {
		t.Errorf("jam weight = %.1f, want ≈%.1f", w[0], 2*base)
	}
	// closure ⇒ +Inf
	w = Customize(g, Metric{Closed: map[EdgeKey]bool{{1, 2}: true}})
	if !math.IsInf(w[0], 1) {
		t.Errorf("closed edge must be +Inf, got %v", w[0])
	}
	// no metric ⇒ base
	w = Customize(g, Metric{})
	if math.Abs(w[0]-base) > 1e-9 {
		t.Errorf("empty metric must equal base")
	}
}

func TestMetricJSONRoundTrip(t *testing.T) {
	m := Metric{
		ClassMult: map[RoadClass]float64{ClassMotorway: 0.8, ClassPrimary: 0.5},
		Closed:    map[EdgeKey]bool{{From: 1, To: 2}: true, {From: 3, To: 4}: true},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m2 Metric
	if err := json.Unmarshal(b, &m2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m2.Closed) != 2 || !m2.Closed[EdgeKey{1, 2}] {
		t.Errorf("closed edges round-trip broken")
	}
	if m2.ClassMult[ClassPrimary] != 0.5 {
		t.Errorf("class mult round-trip broken")
	}
}
