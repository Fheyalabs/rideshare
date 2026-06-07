// Package discovery implements the hierarchical H3 cell discovery: drivers
// heartbeat a coarse cell, riders escalate tight→wide. No exact GPS is stored.
package discovery

import h3 "github.com/uber/h3-go/v4"

// BaseResolution is the tightest cell the rider ever reveals (~150 m edge).
// H3 res 9 ≈ 0.10 km² hexagons, res 8 ≈ 0.46 km². Use res 9 for the tightest.
const BaseResolution = 9

// Cell is an H3 cell index.
type Cell = h3.Cell

// CellAt returns the H3 cell containing the point at the given resolution.
// Panics if the underlying h3-go call returns an error (invalid lat/lng).
func CellAt(lat, lng float64, res int) Cell {
	c, err := h3.LatLngToCell(h3.NewLatLng(lat, lng), res)
	if err != nil {
		panic(err)
	}
	return c
}

// Widen returns the parent (one resolution coarser) — the "no, widen" step.
// Panics if the underlying h3-go call returns an error.
func Widen(c Cell) Cell {
	p, err := c.Parent(c.Resolution() - 1)
	if err != nil {
		panic(err)
	}
	return p
}
