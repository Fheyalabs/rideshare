package movement

import (
	"math/rand"

	"github.com/Fheyalabs/rideshare/internal/discovery"
	"github.com/Fheyalabs/rideshare/internal/routing"
)

// Engine manages N actors roaming over the graph, firing cell-change callbacks.
type Engine struct {
	g            *routing.Graph
	weights      []float64
	rng          *rand.Rand
	actors       map[string]*Actor
	lastCell     map[string]discovery.Cell
	nodeIDs      []int64
	OnCellChange func(id string, cell discovery.Cell)
}

// NewEngine creates an engine with the given graph, customization weights, and PRNG seed.
func NewEngine(g *routing.Graph, weights []float64, seed int64) *Engine {
	ids := make([]int64, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	return &Engine{g: g, weights: weights, rng: rand.New(rand.NewSource(seed)),
		actors: map[string]*Actor{}, lastCell: map[string]discovery.Cell{}, nodeIDs: ids}
}

// SetWeights replaces the live customization weights.
func (e *Engine) SetWeights(w []float64) { e.weights = w }

// Actor returns the named actor or nil.
func (e *Engine) Actor(id string) *Actor { return e.actors[id] }

// Add places a new actor at a start node and gives it an initial roam track.
func (e *Engine) Add(id string, startNode int64) {
	a := NewActor(id, e.g, []int64{startNode})
	e.actors[id] = a
	e.lastCell[id] = a.Cell()
	e.roam(a)
}

func (e *Engine) roam(a *Actor) {
	for i := 0; i < 8; i++ {
		dst := e.nodeIDs[e.rng.Intn(len(e.nodeIDs))]
		if path, _, _, ok := routing.AStar(e.g, a.track[a.seg], dst, e.weights); ok && len(path) > 1 {
			a.SetTrack(path)
			return
		}
	}
}

// SetTarget routes an actor to dst now (manual override).
func (e *Engine) SetTarget(id string, dst int64) {
	a := e.actors[id]
	if a == nil {
		return
	}
	if path, _, _, ok := routing.AStar(e.g, a.track[a.seg], dst, e.weights); ok {
		a.SetTrack(path)
	}
}

// Tick advances every actor by dt seconds and fires cell-change callbacks.
func (e *Engine) Tick(dt float64) {
	for id, a := range e.actors {
		ei := edgeSpeed(e.g, e.weights, a)
		a.Advance(dt, ei)
		if a.Arrived() {
			e.roam(a)
		}
		if c := a.Cell(); c != e.lastCell[id] {
			e.lastCell[id] = c
			if e.OnCellChange != nil {
				e.OnCellChange(id, c)
			}
		}
	}
}

// edgeSpeed returns the actor's current effective speed (m/s) from the live weights.
func edgeSpeed(g *routing.Graph, w []float64, a *Actor) float64 {
	if a.Arrived() {
		return 0
	}
	from, to := a.track[a.seg], a.track[a.seg+1]
	for _, ei := range g.Adj[from] {
		if g.Edges[ei].To == to && w[ei] > 0 {
			return g.Edges[ei].LenM / w[ei]
		}
	}
	return 13.9 // fallback ~50 km/h
}
