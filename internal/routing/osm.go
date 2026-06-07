package routing

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"
)

// WayRec is a drivable way distilled from OSM.
type WayRec struct {
	Nodes       []int64
	Highway     string
	Ref         string
	Oneway      bool
	MaxSpeedKmh float64
}

// BuildGraph turns distilled ways + node coords into a routing graph. Pure +
// unit-testable. Edges are split per consecutive node pair; non-drivable ways
// and ways with missing node coords are skipped.
func BuildGraph(ways []WayRec, coords map[int64][2]float64) *Graph {
	g := NewGraph()
	for _, w := range ways {
		class, ok := Classify(w.Highway)
		if !ok {
			continue
		}
		for _, id := range w.Nodes {
			if c, ok := coords[id]; ok {
				g.AddNode(id, c[0], c[1])
			}
		}
		for i := 0; i+1 < len(w.Nodes); i++ {
			a, b := w.Nodes[i], w.Nodes[i+1]
			if _, ok := g.Nodes[a]; !ok {
				continue
			}
			if _, ok := g.Nodes[b]; !ok {
				continue
			}
			before := len(g.Edges)
			g.AddEdge(a, b, class, w.MaxSpeedKmh, w.Oneway)
			for j := before; j < len(g.Edges); j++ {
				g.Edges[j].Ref = w.Ref
			}
		}
	}
	return g
}

// ParsePBF reads an OSM .pbf and builds the graph (two passes: ways, then the
// node coords they reference).
func ParsePBF(path string) (*Graph, error) {
	ways, nodeIDs, err := scanWays(path)
	if err != nil {
		return nil, err
	}
	coords, err := scanNodes(path, nodeIDs)
	if err != nil {
		return nil, err
	}
	return BuildGraph(ways, coords), nil
}

func scanWays(path string) ([]WayRec, map[int64]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	s := osmpbf.New(context.Background(), f, runtime.GOMAXPROCS(-1))
	s.SkipNodes, s.SkipRelations = true, true
	defer s.Close()
	var ways []WayRec
	need := map[int64]struct{}{}
	for s.Scan() {
		w, ok := s.Object().(*osm.Way)
		if !ok {
			continue
		}
		hw := w.Tags.Find("highway")
		if _, drive := Classify(hw); !drive {
			continue
		}
		ids := make([]int64, len(w.Nodes))
		for i, wn := range w.Nodes {
			ids[i] = int64(wn.ID)
			need[ids[i]] = struct{}{}
		}
		spd, _ := strconv.ParseFloat(w.Tags.Find("maxspeed"), 64)
		ways = append(ways, WayRec{
			Nodes: ids, Highway: hw, Ref: w.Tags.Find("ref"),
			Oneway:      w.Tags.Find("oneway") == "yes",
			MaxSpeedKmh: spd,
		})
	}
	return ways, need, s.Err()
}

func scanNodes(path string, need map[int64]struct{}) (map[int64][2]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := osmpbf.New(context.Background(), f, runtime.GOMAXPROCS(-1))
	s.SkipWays, s.SkipRelations = true, true
	defer s.Close()
	coords := make(map[int64][2]float64, len(need))
	for s.Scan() {
		n, ok := s.Object().(*osm.Node)
		if !ok {
			continue
		}
		if _, want := need[int64(n.ID)]; want {
			coords[int64(n.ID)] = [2]float64{n.Lat, n.Lon}
		}
	}
	if s.Err() != nil {
		return nil, fmt.Errorf("scan nodes: %w", s.Err())
	}
	return coords, nil
}
