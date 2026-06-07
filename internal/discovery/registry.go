package discovery

import "sync"

// Registry is a concurrency-safe map of driver pseudonym → base H3 cell.
type Registry struct {
	mu     sync.RWMutex
	driver map[string]Cell // pseudonym → base cell
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{driver: make(map[string]Cell)}
}

// SetCell records (or updates) a driver's current coarse cell. Drivers call this
// on a grid-change event (event-driven, no periodic heartbeat); call Remove when
// going off-duty / mid-ride.
func (r *Registry) SetCell(pseudonym string, cell Cell) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.driver[pseudonym] = cell
}

// Remove deletes a driver from the registry.
func (r *Registry) Remove(pseudonym string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.driver, pseudonym)
}

// Discover escalates from start, widening up to maxWiden times, returning the
// candidate pseudonyms within the (possibly widened) query cell and the final cell.
func (r *Registry) Discover(start Cell, target, maxWiden int) ([]string, Cell) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	q := start
	for w := 0; ; w++ {
		var cands []string
		for p, c := range r.driver {
			if contains(q, c) {
				cands = append(cands, p)
			}
		}
		if len(cands) >= target || w >= maxWiden {
			return cands, q
		}
		q = Widen(q)
	}
}

// contains reports whether base-resolution cell c is within query cell q
// (q is c, or an ancestor of c).
func contains(q, c Cell) bool {
	if q == c {
		return true
	}
	p, err := c.Parent(q.Resolution())
	if err != nil {
		return false
	}
	return p == q
}
