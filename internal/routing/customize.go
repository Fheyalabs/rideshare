package routing

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// EdgeKey identifies an edge by endpoints (directed).
type EdgeKey struct{ From, To int64 }

// Metric is the customization input: speed multipliers (per class and per edge,
// where 1.0 = free-flow, <1 = slower/jam) and a set of closed edges. It is the
// small payload the server broadcasts and the client applies — no re-contraction.
type Metric struct {
	ClassMult map[RoadClass]float64
	EdgeMult  map[EdgeKey]float64
	Closed    map[EdgeKey]bool
}

func (m Metric) mult(e Edge) float64 {
	v := 1.0
	if m.ClassMult != nil {
		if cm, ok := m.ClassMult[e.Class]; ok {
			v = cm
		}
	}
	if m.EdgeMult != nil {
		if em, ok := m.EdgeMult[EdgeKey{e.From, e.To}]; ok {
			v = em
		}
	}
	return v
}

// Customize returns per-edge current traversal time (seconds), parallel to
// g.Edges: base / speedMultiplier, or +Inf if closed. Level-free re-weight.
func Customize(g *Graph, m Metric) []float64 {
	out := make([]float64, len(g.Edges))
	for i, e := range g.Edges {
		if m.Closed != nil && m.Closed[EdgeKey{e.From, e.To}] {
			out[i] = math.Inf(1)
			continue
		}
		mul := m.mult(e)
		if mul <= 0 {
			out[i] = math.Inf(1)
			continue
		}
		out[i] = g.BaseWeightSec(i) / mul
	}
	return out
}

// MarshalJSON encodes Metric with EdgeMult/Closed as "from:to" strings.
func (m Metric) MarshalJSON() ([]byte, error) {
	type wire struct {
		ClassMult map[RoadClass]float64 `json:"class_mult,omitempty"`
		EdgeMult  map[string]float64    `json:"edge_mult,omitempty"`
		Closed    []string              `json:"closed,omitempty"`
	}
	w := wire{ClassMult: m.ClassMult}
	if len(m.EdgeMult) > 0 {
		w.EdgeMult = map[string]float64{}
		for k, v := range m.EdgeMult {
			w.EdgeMult[ekStr(k)] = v
		}
	}
	for k, on := range m.Closed {
		if on {
			w.Closed = append(w.Closed, ekStr(k))
		}
	}
	return json.Marshal(w)
}

// UnmarshalJSON decodes Metric from the "from:to" string form.
func (m *Metric) UnmarshalJSON(b []byte) error {
	var w struct {
		ClassMult map[RoadClass]float64 `json:"class_mult"`
		EdgeMult  map[string]float64    `json:"edge_mult"`
		Closed    []string              `json:"closed"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	m.ClassMult = w.ClassMult
	if len(w.EdgeMult) > 0 {
		m.EdgeMult = map[EdgeKey]float64{}
		for s, v := range w.EdgeMult {
			if k, ok := ekParse(s); ok {
				m.EdgeMult[k] = v
			}
		}
	}
	if len(w.Closed) > 0 {
		m.Closed = map[EdgeKey]bool{}
		for _, s := range w.Closed {
			if k, ok := ekParse(s); ok {
				m.Closed[k] = true
			}
		}
	}
	return nil
}

func ekStr(k EdgeKey) string {
	return strconv.FormatInt(k.From, 10) + ":" + strconv.FormatInt(k.To, 10)
}

func ekParse(s string) (EdgeKey, bool) {
	var f, t int64
	if _, err := fmt.Sscanf(s, "%d:%d", &f, &t); err != nil {
		return EdgeKey{}, false
	}
	return EdgeKey{From: f, To: t}, true
}
