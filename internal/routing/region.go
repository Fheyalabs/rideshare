package routing

import (
	"encoding/json"
	"sort"
)

type wireGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Serialize encodes the graph for client download (cached region). JSON for
// cross-language clients; swap to a compact binary later if size demands.
// Nodes are sorted by ID for deterministic output (stable ETags, reproducible artifacts).
func Serialize(g *Graph) ([]byte, error) {
	w := wireGraph{Edges: g.Edges}
	for _, n := range g.Nodes {
		w.Nodes = append(w.Nodes, n)
	}
	sort.Slice(w.Nodes, func(i, j int) bool { return w.Nodes[i].ID < w.Nodes[j].ID })
	return json.Marshal(w)
}

// Deserialize decodes a serialized graph.
func Deserialize(b []byte) (*Graph, error) {
	var w wireGraph
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	g := NewGraph()
	for _, n := range w.Nodes {
		g.Nodes[n.ID] = n
	}
	for _, e := range w.Edges {
		idx := len(g.Edges)
		g.Edges = append(g.Edges, e)
		g.Adj[e.From] = append(g.Adj[e.From], idx)
	}
	return g, nil
}

// Slice extracts the subgraph whose edges have both endpoints in keep. Used to
// serve a decoy-padded batch (the client supplies real + arbitrary node sets;
// the server cannot tell which is real).
func Slice(g *Graph, keep map[int64]bool) *Graph {
	s := NewGraph()
	for id := range keep {
		if n, ok := g.Nodes[id]; ok {
			s.Nodes[id] = n
		}
	}
	for _, e := range g.Edges {
		if keep[e.From] && keep[e.To] {
			idx := len(s.Edges)
			s.Edges = append(s.Edges, e)
			s.Adj[e.From] = append(s.Adj[e.From], idx)
		}
	}
	return s
}
