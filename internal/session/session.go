// Package session orchestrates the single-key blind auction session.
// The server collects the rider's public key and each driver's signed encrypted
// bid, then runs the blind auction — it NEVER holds a secret key.
package session

import (
	"fmt"
	"sync"

	"github.com/Fheyalabs/rideshare/internal/auction"
	"github.com/Fheyalabs/rideshare/internal/rating"
)

// ContractParams mirrors cgo.ContractParams for pure-Go compilation.
type ContractParams struct {
	RingDim       uint32
	Depth         uint32
	ScalingFactor float64
}

// AuctionWeights mirrors cgo.AuctionWeights for pure-Go compilation.
type AuctionWeights struct{ K, WStar, WDist float64 }

// DefaultWeights returns lexicographic weights (no location term — WDist=0).
func DefaultWeights() AuctionWeights { return AuctionWeights{K: 100, WStar: 1, WDist: 0} }

// DefaultParams returns ring 2^15, depth 5, scaling 2^50.
func DefaultParams() ContractParams {
	return ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
}

// AuctionSession collects the rider pk + signed driver bids and runs the blind
// argmin. The server never sees a secret key. ★ is server-authoritative (looked
// up from the rating store) — drivers cannot self-report.
type AuctionSession struct {
	mu      sync.Mutex
	id      []byte
	params  ContractParams
	band    auction.PriceBand
	w       AuctionWeights
	degree  int
	ratings *rating.Store

	riderPK []byte
	bids    []heldBid
}

type heldBid struct {
	sb   auction.SignedBid
	star float64
}

// NewAuctionSession creates a session ready to accept bids.
func NewAuctionSession(
	id []byte,
	params ContractParams,
	band auction.PriceBand,
	w AuctionWeights,
	degree int,
	ratings *rating.Store,
) *AuctionSession {
	return &AuctionSession{id: id, params: params, band: band, w: w, degree: degree, ratings: ratings}
}

// SetRiderPK stores the rider's public key (received once per session).
func (s *AuctionSession) SetRiderPK(pk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riderPK = pk
}

// SubmitBid stores a verified signed bid. ★ is looked up from the rating store
// — the driver cannot self-report (spec §5.5). The location term is dropped
// (WDist=0, dist=0 — no ARES-core change).
func (s *AuctionSession) SubmitBid(sb auction.SignedBid) error {
	if err := sb.Verify(s.id); err != nil {
		return fmt.Errorf("bid signature invalid: %w", err)
	}
	star := 4.3 // global mean fallback
	if s.ratings != nil {
		star = s.ratings.StarNorm(string(sb.Pubkey))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bids = append(s.bids, heldBid{sb: sb, star: star})
	return nil
}

// BidCount returns the current number of submitted bids.
func (s *AuctionSession) BidCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bids)
}

// PseudonymAt returns the driver pseudonym (derived from pubkey) at index i.
func (s *AuctionSession) PseudonymAt(i int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i < 0 || i >= len(s.bids) {
		return ""
	}
	return string(s.bids[i].sb.Pubkey)
}

// EncodeCGO returns the session state encoded for the cgo bridge.
func (s *AuctionSession) EncodeCGO() (pk []byte, encBids [][]byte, stars, dists []float64, nonces [][]byte, band struct{ Floor, Cap int }, w struct{ K, WStar, WDist float64 }, degree int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.bids)
	encBids = make([][]byte, n)
	stars = make([]float64, n)
	dists = make([]float64, n) // always zero — location term dropped
	nonces = make([][]byte, n)
	for i, b := range s.bids {
		encBids[i] = b.sb.EncBid
		stars[i] = b.star
		nonces[i] = b.sb.Nonce
	}
	pk = s.riderPK
	band.Floor = s.band.FloorCents
	band.Cap = s.band.CapCents
	w.K = s.w.K
	w.WStar = s.w.WStar
	w.WDist = s.w.WDist
	degree = s.degree
	return
}
