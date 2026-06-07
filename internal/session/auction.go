//go:build openfhe

package session

import (
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
)

// RunBlindAuction runs the server-side auction on collected encrypted bids.
// The server never sees a secret key — it only gets back encrypted masks.
// Returns serialized encrypted mask ciphertexts (one per driver).
func (s *AuctionSession) RunBlindAuction() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.riderPK) == 0 {
		return nil, fmt.Errorf("rider pk not set")
	}
	n := len(s.bids)
	if n < 2 {
		return nil, fmt.Errorf("need >= 2 bids, got %d", n)
	}

	encBids := make([][]byte, n)
	stars := make([]float64, n)
	dists := make([]float64, n)
	nonces := make([][]byte, n)
	for i, b := range s.bids {
		encBids[i] = b.sb.EncBid
		stars[i] = b.star
		dists[i] = b.dist
		nonces[i] = b.sb.Nonce
	}

	return cgo.SingleKeyAuctionServerEnc(
		s.params, s.riderPK, encBids, stars, dists, nonces,
		s.band.FloorCents, s.band.CapCents,
		s.w, s.degree,
	)
}
