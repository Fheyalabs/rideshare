// Package session orchestrates the single-key blind auction session.
// The server collects the rider's public key and each driver's signed encrypted
// bid, then runs the blind auction — it NEVER holds a secret key.
package session

import (
	"fmt"
	"sync"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/rideshare/internal/auction"
)

// AuctionSession collects the rider pk + signed driver bids and runs the blind
// argmin. The server never sees a secret key.
type AuctionSession struct {
	mu     sync.Mutex
	id     []byte
	params cgo.ContractParams
	band   auction.PriceBand
	w      cgo.AuctionWeights
	degree int

	riderPK []byte
	bids    []heldBid
}

type heldBid struct {
	sb   auction.SignedBid
	star float64
	dist float64
}

// NewAuctionSession creates a session ready to accept bids.
func NewAuctionSession(
	id []byte,
	params cgo.ContractParams,
	band auction.PriceBand,
	w cgo.AuctionWeights,
	degree int,
) *AuctionSession {
	return &AuctionSession{id: id, params: params, band: band, w: w, degree: degree}
}

// SetRiderPK stores the rider's public key (received once per session).
func (s *AuctionSession) SetRiderPK(pk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.riderPK = pk
}

// SubmitBid stores a verified signed bid. The signature is checked before
// storage — ghost bids from the server are rejected at submit time.
func (s *AuctionSession) SubmitBid(sb auction.SignedBid, star, dist float64) error {
	if err := sb.Verify(s.id); err != nil {
		return fmt.Errorf("bid signature invalid: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bids = append(s.bids, heldBid{sb: sb, star: star, dist: dist})
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
