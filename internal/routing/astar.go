package routing

import (
	"container/heap"
	"math"
)

// maxGraphSpeedMps bounds the A* time-heuristic (admissible: never overestimates).
const maxGraphSpeedMps = 140.0 / 3.6 // ~140 km/h

// AStar finds the minimum-time path from→to using per-edge weights (seconds,
// parallel to g.Edges, e.g. from Customize). Returns the node path, total metres,
// total seconds, and ok=false if unreachable.
func AStar(g *Graph, from, to int64, weights []float64) ([]int64, float64, float64, bool) {
	if from == to {
		return []int64{from}, 0, 0, true
	}
	dst, ok := g.Nodes[to]
	if !ok {
		return nil, 0, 0, false
	}
	gScore := map[int64]float64{from: 0}
	prev := map[int64]int64{}
	prevEdge := map[int64]int{}
	open := &pq{}
	heap.Init(open)
	heap.Push(open, item{node: from, f: heuristic(g.Nodes[from], dst)})
	seen := map[int64]bool{}

	for open.Len() > 0 {
		cur := heap.Pop(open).(item)
		if cur.node == to {
			return reconstruct(g, prev, prevEdge, from, to, weights)
		}
		if seen[cur.node] {
			continue
		}
		seen[cur.node] = true
		for _, ei := range g.Adj[cur.node] {
			e := g.Edges[ei]
			w := weights[ei]
			if math.IsInf(w, 1) {
				continue // closed edge
			}
			cand := gScore[cur.node] + w
			if old, ok := gScore[e.To]; !ok || cand < old {
				gScore[e.To] = cand
				prev[e.To] = cur.node
				prevEdge[e.To] = ei
				heap.Push(open, item{node: e.To, f: cand + heuristic(g.Nodes[e.To], dst)})
			}
		}
	}
	return nil, 0, 0, false
}

func heuristic(a, b Node) float64 {
	return DistM(a.Lat, a.Lon, b.Lat, b.Lon) / maxGraphSpeedMps
}

func reconstruct(g *Graph, prev map[int64]int64, prevEdge map[int64]int, from, to int64, w []float64) ([]int64, float64, float64, bool) {
	var path []int64
	var distM, durS float64
	for n := to; n != from; n = prev[n] {
		path = append([]int64{n}, path...)
		ei := prevEdge[n]
		distM += g.Edges[ei].LenM
		durS += w[ei]
	}
	path = append([]int64{from}, path...)
	return path, distM, durS, true
}

type item struct {
	node int64
	f    float64
}
type pq []item

func (p pq) Len() int            { return len(p) }
func (p pq) Less(i, j int) bool  { return p[i].f < p[j].f }
func (p pq) Swap(i, j int)       { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)         { *p = append(*p, x.(item)) }
func (p *pq) Pop() any           { o := *p; n := len(o); it := o[n-1]; *p = o[:n-1]; return it }
