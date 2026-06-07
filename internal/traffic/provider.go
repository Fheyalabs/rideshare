// Package traffic supplies the routing customization metric: a deterministic
// time-of-day model now, plus free Autobahn closures, behind a TrafficProvider
// interface so a HERE/TomTom live-flow impl drops in later.
package traffic

import (
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

// TrafficProvider yields the current customization metric for time t.
type TrafficProvider interface {
	Metric(t time.Time) routing.Metric
}

// SimulatedProvider is a deterministic time-of-day model: arterials slow in the
// AM/PM rush, mild at night; motorways less affected than city streets.
type SimulatedProvider struct{}

func (SimulatedProvider) Metric(t time.Time) routing.Metric {
	h := t.Hour()
	cityMul, hwyMul := 1.0, 1.0
	switch {
	case h >= 7 && h < 10: // AM rush
		cityMul, hwyMul = 0.55, 0.8
	case h >= 16 && h < 19: // PM rush
		cityMul, hwyMul = 0.5, 0.75
	case h >= 22 || h < 5: // night: free-flowing
		cityMul, hwyMul = 1.0, 1.0
	default:
		cityMul, hwyMul = 0.85, 0.95
	}
	return routing.Metric{ClassMult: map[routing.RoadClass]float64{
		routing.ClassMotorway:  hwyMul,
		routing.ClassTrunk:     hwyMul,
		routing.ClassPrimary:   cityMul,
		routing.ClassSecondary: cityMul,
		routing.ClassTertiary:  cityMul,
		routing.ClassLocal:     cityMul,
	}}
}
