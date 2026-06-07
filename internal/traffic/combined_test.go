package traffic

import (
	"testing"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

func TestCombined_MergesClosures(t *testing.T) {
	c := CombinedProvider{
		Base: SimulatedProvider{},
		Closure: func() map[routing.EdgeKey]bool {
			return map[routing.EdgeKey]bool{{From: 1, To: 2}: true}
		},
	}
	m := c.Metric(time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC))
	if !m.Closed[routing.EdgeKey{From: 1, To: 2}] {
		t.Error("closure not merged into metric")
	}
	if m.ClassMult == nil {
		t.Error("base class multipliers missing")
	}
}
