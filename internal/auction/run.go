//go:build openfhe

package auction

import (
	"crypto/sha256"
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
)

// RunAuction runs the single-key reverse auction end to end:
//
//	rider keygen → each driver encrypts its OWN price under the rider's pk
//	→ server runs the blind argmax on the encrypted bids (never sees plaintext)
//	→ rider decrypts the masks (sole decryptor) → winner.
//
// The agreed price is recovered by the rider DECRYPTING THE WINNER'S OWN bid
// ciphertext — so the revealed price IS the committed bid (binding).
// Returns the winner package and the shared secret SHA256(nonce_winner || sessionID)
// used for the pickup recognition phrase / OTP.
func RunAuction(params cgo.ContractParams, bids []DriverBid, nonces [][]byte,
	w Weights, band PriceBand, degree int, sessionID []byte) (WinnerPackage, []byte, error) {

	n := len(bids)
	if n < 2 || n > MaxDrivers+4 { // single-key supports up to 9 at ring 2^15
		return WinnerPackage{}, nil, fmt.Errorf("RunAuction: n=%d out of range", n)
	}
	if len(nonces) != n {
		return WinnerPackage{}, nil, fmt.Errorf("RunAuction: %d nonces for %d bids", len(nonces), n)
	}

	pk, sk, err := cgo.SingleKeyGen(params)
	if err != nil {
		return WinnerPackage{}, nil, fmt.Errorf("keygen: %w", err)
	}

	// Each driver encrypts its own price under the rider's pk (server stays blind).
	encBids := make([][]byte, n)
	stars := make([]float64, n)
	dists := make([]float64, n)
	for i, b := range bids {
		ct, err := cgo.SingleKeyEncrypt(params, pk, float64(b.PriceCents))
		if err != nil {
			return WinnerPackage{}, nil, fmt.Errorf("encrypt bid[%d]: %w", i, err)
		}
		encBids[i] = ct
		stars[i] = b.StarNorm
		dists[i] = b.DistSq
	}

	cw := cgo.AuctionWeights{K: w.K, WStar: w.WStar, WDist: w.WDist}
	masksCt, err := cgo.SingleKeyAuctionServerEnc(params, pk, encBids, stars, dists, nonces,
		band.FloorCents, band.CapCents, cw, degree)
	if err != nil {
		return WinnerPackage{}, nil, fmt.Errorf("auction server: %w", err)
	}

	_, winner, err := cgo.SingleKeyAuctionDecrypt(params, sk, masksCt)
	if err != nil {
		return WinnerPackage{}, nil, fmt.Errorf("decrypt masks: %w", err)
	}
	if winner < 0 || winner >= n {
		return WinnerPackage{}, nil, fmt.Errorf("invalid winner index %d", winner)
	}

	// Binding: recover the agreed price by decrypting the WINNER'S committed bid ct.
	agreed, err := cgo.SingleKeyDecrypt(params, sk, encBids[winner], 4)
	if err != nil {
		return WinnerPackage{}, nil, fmt.Errorf("decrypt agreed price: %w", err)
	}
	agreedCents := int(agreed[0] + 0.5)

	// Shared secret for the pickup phrase / OTP (§5.7).
	h := sha256.New()
	h.Write(nonces[winner])
	h.Write(sessionID)
	secret := h.Sum(nil)

	return WinnerPackage{
		WinnerIndex:      winner,
		Pseudonym:        bids[winner].Pseudonym,
		AgreedPriceCents: agreedCents,
		StarNorm:         bids[winner].StarNorm,
	}, secret, nil
}
