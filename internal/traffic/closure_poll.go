package traffic

import (
	"log"
	"sync"
	"time"

	"github.com/Fheyalabs/rideshare/internal/routing"
)

// ClosurePoller periodically fetches Autobahn closures for the given road refs,
// matches them against the graph, and updates a cached closure set consumed by
// CombinedProvider. Call Start() to begin the background goroutine.
type ClosurePoller struct {
	mu       sync.RWMutex
	closed   map[routing.EdgeKey]bool
	baseURL  string // Autobahn API base (empty = no-op)
	refs     []string
	interval time.Duration
	graph    *routing.Graph
	radiusM  float64
}

// NewClosurePoller returns a poller. baseURL empty means closures disabled.
func NewClosurePoller(baseURL string, refs []string, graph *routing.Graph, interval time.Duration, radiusM float64) *ClosurePoller {
	return &ClosurePoller{
		closed:   map[routing.EdgeKey]bool{},
		baseURL:  baseURL,
		refs:     refs,
		interval: interval,
		graph:    graph,
		radiusM:  radiusM,
	}
}

// Closed returns the latest cached closure set (thread-safe).
func (p *ClosurePoller) Closed() map[routing.EdgeKey]bool {
	if p == nil || p.baseURL == "" {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[routing.EdgeKey]bool, len(p.closed))
	for k := range p.closed {
		out[k] = true
	}
	return out
}

// Start begins the background poll loop. Call once.
func (p *ClosurePoller) Start() {
	if p == nil || p.baseURL == "" {
		return
	}
	go func() {
		p.poll() // initial fetch
		tick := time.NewTicker(p.interval)
		defer tick.Stop()
		for range tick.C {
			p.poll()
		}
	}()
}

func (p *ClosurePoller) poll() {
	closed := map[routing.EdgeKey]bool{}
	for _, ref := range p.refs {
		pts, err := FetchClosures(p.baseURL, ref)
		if err != nil {
			log.Printf("[closures] fetch %s: %v", ref, err)
			continue
		}
		for k := range MatchClosures(p.graph, pts, ref, p.radiusM) {
			closed[k] = true
		}
	}
	p.mu.Lock()
	p.closed = closed
	p.mu.Unlock()
}
