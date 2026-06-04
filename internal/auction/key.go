// Package auction implements rideshare's single-key reverse auction — the only
// homomorphic circuit in the app. Drivers submit one encrypted, Ed25519-signed
// price under the rider's single-key CKKS public key; the server runs a blind
// soft-mask argmin over the server-built lexicographic key (price → ★ → dist²,
// with ★ and dist² server-authoritative); the rider alone decrypts the masks,
// recovers the agreed price from the winner's own committed bid (binding), and
// verifies the winning driver's signature (ghost-driver rejection). This file
// holds the plaintext ranking contract (Weights/BuildKey); signing.go the
// Ed25519 binding; run.go (build tag openfhe) the real CKKS flow via ARES-core's
// SingleKey* primitives.
package auction

// Weights parameterise the lexicographic key. K must exceed the maximum
// possible tiebreak magnitude so price strictly dominates (see spec §5.5).
type Weights struct {
	K     float64 // price multiplier (dominant term)
	WStar float64 // ★-penalty weight (secondary)
	WDist float64 // dist² weight (tertiary)
}

// BuildKey assembles the lexicographic key for one driver. Lower = better.
//   priceCents:    the committed bid in cents (integer).
//   starPenalty:   5 − normalized★ (so a worse rating raises the key).
//   distSq:        squared coarse pickup-leg distance (server-authoritative).
// Invariant: WStar·maxStarPenalty + WDist·maxDistSq < K, so a 1-cent price
// difference outranks any tiebreak.
func BuildKey(priceCents int, starPenalty, distSq float64, w Weights) float64 {
	return w.K*float64(priceCents) + w.WStar*starPenalty + w.WDist*distSq
}
