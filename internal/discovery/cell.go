// Package discovery implements hierarchical H3 cell discovery. Clients own the
// grid math: a driver pushes its coarse cell on a grid-change event (no periodic
// heartbeat), and a rider queries with its own coarse cell. The server only ever
// sees H3 cell ids — never raw coordinates — so it never learns exact GPS.
package discovery

import (
	"fmt"

	h3 "github.com/uber/h3-go/v4"
)

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

// CellToString returns the canonical H3 string id for a cell — the wire form
// clients and server exchange. No coordinates ever cross the boundary.
func CellToString(c Cell) string { return h3.IndexToString(uint64(c)) }

// CellFromString parses a canonical H3 string id into a Cell, returning an error
// for malformed or invalid input so the server can reject bad client data.
func CellFromString(s string) (Cell, error) {
	c := Cell(h3.IndexFromString(s))
	if !c.IsValid() {
		return 0, fmt.Errorf("invalid H3 cell %q", s)
	}
	return c, nil
}
