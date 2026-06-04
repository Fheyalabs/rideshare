// Package auction holds the reverse-auction logic: the lexicographic
// ranking key (this file, plaintext — the contract the FHE reproduces)
// and the real-FHE argmin (run.go).
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
