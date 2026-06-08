// Package movement simulates actor motion over the routing graph: actors follow
// a node track, expose synthetic GPS (LatLng) + their H3 cell, and signal cell
// changes so a ghost driver can push /grid. No real GPS is used.
package movement

import (
	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/routing"
)

// Actor moves along a track (node sequence) over the graph.
type Actor struct {
	ID    string
	g     *routing.Graph
	track []int64
	seg   int
	frac  float64
}

// NewActor creates an actor at the start of the given track.
func NewActor(id string, g *routing.Graph, track []int64) *Actor {
	return &Actor{ID: id, g: g, track: track}
}

// SetTrack replaces the actor's track and resets progress.
func (a *Actor) SetTrack(track []int64) { a.track, a.seg, a.frac = track, 0, 0 }

// Arrived reports whether the actor reached the end of its track.
func (a *Actor) Arrived() bool { return a.track != nil && a.seg >= len(a.track)-1 }

// Advance moves the actor forward by dt seconds at speedMps along its track.
func (a *Actor) Advance(dt, speedMps float64) {
	remaining := dt * speedMps
	for remaining > 0 && !a.Arrived() {
		from := a.g.Nodes[a.track[a.seg]]
		to := a.g.Nodes[a.track[a.seg+1]]
		segLen := routing.DistM(from.Lat, from.Lon, to.Lat, to.Lon)
		if segLen < 1e-6 {
			a.seg++
			a.frac = 0
			continue
		}
		distToEnd := (1 - a.frac) * segLen
		if remaining >= distToEnd {
			remaining -= distToEnd
			a.seg++
			a.frac = 0
		} else {
			a.frac += remaining / segLen
			remaining = 0
		}
	}
}

// LatLng returns the actor's interpolated position. Returns 0,0 if the track
// is empty (actor has not been assigned a route).
func (a *Actor) LatLng() (float64, float64) {
	if a.track == nil || len(a.track) == 0 {
		return 0, 0
	}
	if a.Arrived() {
		n := a.g.Nodes[a.track[len(a.track)-1]]
		return n.Lat, n.Lon
	}
	from := a.g.Nodes[a.track[a.seg]]
	to := a.g.Nodes[a.track[a.seg+1]]
	return from.Lat + (to.Lat-from.Lat)*a.frac, from.Lon + (to.Lon-from.Lon)*a.frac
}

// Cell returns the actor's current coarse H3 cell.
func (a *Actor) Cell() discovery.Cell {
	lat, lon := a.LatLng()
	return discovery.CellAt(lat, lon, discovery.BaseResolution)
}
