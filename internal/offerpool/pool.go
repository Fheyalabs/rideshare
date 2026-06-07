// Package offerpool manages the held-offer lifecycle: max 3 concurrent,
// async expiry, accept-ends-all, winner-excluded from re-search.
package offerpool

import (
	"fmt"
	"sync"
	"time"
)

// Offer is one held auction result.
type Offer struct {
	ID         string
	Driver     string
	PriceCents int
	Star       float64
	ExpiresAt  time.Time
}

// Pool manages held offers for one rider's ride request.
type Pool struct {
	mu   sync.Mutex
	max  int
	ttl  time.Duration
	held map[string]Offer // id → Offer
}

// New returns a Pool with the given capacity and per-offer TTL.
// A background goroutine sweeps expired offers every second.
func New(max int, ttl time.Duration) *Pool {
	p := &Pool{max: max, ttl: ttl, held: make(map[string]Offer)}
	go func() {
		for range time.Tick(time.Second) {
			p.sweep()
		}
	}()
	return p
}

// Hold adds an offer. Returns an error if the pool is full.
func (p *Pool) Hold(o Offer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.held) >= p.max {
		return fmt.Errorf("offer pool full (%d held)", len(p.held))
	}
	o.ExpiresAt = time.Now().Add(p.ttl)
	p.held[o.ID] = o
	return nil
}

// Held returns currently-active (non-expired) held offers.
func (p *Pool) Held() []Offer {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Offer, 0, len(p.held))
	now := time.Now()
	for _, o := range p.held {
		if now.Before(o.ExpiresAt) {
			out = append(out, o)
		} else {
			delete(p.held, o.ID)
		}
	}
	return out
}

// Accept ends the negotiation: all held offers are released.
func (p *Pool) Accept(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.held[id]; !ok {
		return fmt.Errorf("offer %s not found", id)
	}
	p.held = make(map[string]Offer)
	return nil
}

// Cancel removes one held offer (rider action). Drivers cannot cancel.
func (p *Pool) Cancel(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.held[id]; !ok {
		return fmt.Errorf("offer %s not found", id)
	}
	delete(p.held, id)
	return nil
}

// Excluded reports whether a driver has any held offer (winner-excluded from re-search).
func (p *Pool) Excluded(driver string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, o := range p.held {
		if o.Driver == driver {
			return true
		}
	}
	return false
}

func (p *Pool) sweep() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	for id, o := range p.held {
		if now.After(o.ExpiresAt) {
			delete(p.held, id)
		}
	}
}
