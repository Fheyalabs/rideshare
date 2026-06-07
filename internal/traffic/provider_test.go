package traffic

import (
	"testing"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

func TestSimulated_RushHourSlowsArterials(t *testing.T) {
	p := SimulatedProvider{}
	rush := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) // 08:00 Mon
	night := time.Date(2026, 6, 8, 3, 0, 0, 0, time.UTC)
	mr := p.Metric(rush)
	mn := p.Metric(night)
	if mr.ClassMult[routing.ClassPrimary] >= mn.ClassMult[routing.ClassPrimary] {
		t.Error("rush hour must slow primaries more than night")
	}
	if mr.ClassMult[routing.ClassPrimary] >= 1.0 {
		t.Error("rush multiplier must be <1 (slower)")
	}
}
