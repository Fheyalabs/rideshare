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
	pk, encBids, stars, dists, nonces, band, w, degree := s.EncodeCGO()

	if len(pk) == 0 {
		return nil, fmt.Errorf("rider pk not set")
	}
	n := len(encBids)
	if n < 2 {
		return nil, fmt.Errorf("need >= 2 bids, got %d", n)
	}

	cgoParams := cgo.ContractParams{
		RingDim:       s.params.RingDim,
		Depth:         s.params.Depth,
		ScalingFactor: s.params.ScalingFactor,
	}
	cgoW := cgo.AuctionWeights{K: w.K, WStar: w.WStar, WDist: w.WDist}

	return cgo.SingleKeyAuctionServerEnc(
		cgoParams, pk, encBids, stars, dists, nonces,
		band.Floor, band.Cap, cgoW, degree,
	)
}
