package rating

import "sync"

type rec struct {
	mean  float64
	count int
}

// Store maps a driver identity to its Bayesian-shrunk ★. Server-authoritative —
// drivers cannot self-report (spec §5.5).
type Store struct {
	mu         sync.RWMutex
	recs       map[string]rec
	globalMean float64
	confidence float64
}

// NewStore returns a Store with the given global prior.
func NewStore(globalMean, confidence float64) *Store {
	return &Store{recs: make(map[string]rec), globalMean: globalMean, confidence: confidence}
}

// Record stores a driver's rating data.
func (s *Store) Record(identity string, mean float64, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs[identity] = rec{mean: mean, count: count}
}

// StarNorm returns the Bayesian-shrunk ★ for a driver, or the global mean if unknown.
func (s *Store) StarNorm(identity string) float64 {
	s.mu.RLock()
	r, ok := s.recs[identity]
	s.mu.RUnlock()
	if !ok {
		return s.globalMean
	}
	return Normalize(r.mean, r.count, s.globalMean, s.confidence)
}
