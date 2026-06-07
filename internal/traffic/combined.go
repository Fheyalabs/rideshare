package traffic

import (
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

// CombinedProvider merges a base TrafficProvider (e.g. Simulated) with
// Autobahn closure edges so /customization broadcasts live closures.
type CombinedProvider struct {
	Base    TrafficProvider
	Closure func() map[routing.EdgeKey]bool // called each Metric(); lazy
}

// Metric returns the base metric with closure edges set to +Inf.
func (c CombinedProvider) Metric(t time.Time) routing.Metric {
	m := c.Base.Metric(t)
	if c.Closure != nil {
		closed := c.Closure()
		if len(closed) > 0 {
			if m.Closed == nil {
				m.Closed = closed
			} else {
				for k := range closed {
					m.Closed[k] = true
				}
			}
		}
	}
	return m
}
