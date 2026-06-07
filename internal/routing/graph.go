package routing

import "math"

// Node is an OSM node with coordinates.
type Node struct {
	ID       int64
	Lat, Lon float64
}

// Edge is a directed road segment. Ref is the OSM `ref` tag (e.g. "A17") used to
// match external closures (Autobahn API). MaxSpeedKmh overrides the class default.
type Edge struct {
	From, To    int64
	LenM        float64
	Class       RoadClass
	MaxSpeedKmh float64
	Ref         string
}

// Graph is a directed weighted road graph. Adj maps a node ID to indices into Edges.
type Graph struct {
	Nodes map[int64]Node
	Edges []Edge
	Adj   map[int64][]int
}

// NewGraph returns an empty graph.
func NewGraph() *Graph {
	return &Graph{Nodes: map[int64]Node{}, Edges: nil, Adj: map[int64][]int{}}
}

// AddNode adds a node.
func (g *Graph) AddNode(id int64, lat, lon float64) {
	g.Nodes[id] = Node{ID: id, Lat: lat, Lon: lon}
}

// AddEdge adds a directed edge (and its reverse unless oneway), computing length
// from node coordinates. speedKmh<=0 falls back to the class default.
func (g *Graph) AddEdge(from, to int64, class RoadClass, speedKmh float64, oneway bool) {
	a, ok1 := g.Nodes[from]
	b, ok2 := g.Nodes[to]
	if !ok1 || !ok2 {
		return
	}
	if speedKmh <= 0 {
		speedKmh = DefaultSpeedKmh(class)
	}
	length := DistM(a.Lat, a.Lon, b.Lat, b.Lon)
	g.add(from, to, length, class, speedKmh)
	if !oneway {
		g.add(to, from, length, class, speedKmh)
	}
}

func (g *Graph) add(from, to int64, length float64, class RoadClass, speedKmh float64) {
	idx := len(g.Edges)
	g.Edges = append(g.Edges, Edge{From: from, To: to, LenM: length, Class: class, MaxSpeedKmh: speedKmh})
	g.Adj[from] = append(g.Adj[from], idx)
}

// BaseWeightSec is the free-flow traversal time (seconds) of edge i.
func (g *Graph) BaseWeightSec(i int) float64 {
	e := g.Edges[i]
	mps := e.MaxSpeedKmh / 3.6
	if mps <= 0 {
		return math.Inf(1)
	}
	return e.LenM / mps
}

// DistM returns the haversine distance in metres between two coordinates.
func DistM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	p := math.Pi / 180
	dLat := (lat2 - lat1) * p
	dLon := (lon2 - lon1) * p
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*p)*math.Cos(lat2*p)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
