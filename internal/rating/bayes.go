// Package rating computes the server-authoritative, count-normalized ★
// score. Drivers never self-report ★; the server shrinks the raw mean
// toward the global mean by rating count so few-but-high ratings don't
// outrank many-and-solid ones (spec §5.5).
package rating

// Normalize returns the Bayesian-shrunk rating:
//   (confidence·globalMean + count·mean) / (confidence + count)
// confidence is the pseudo-count weight (how many "global-mean" votes a
// new driver starts with). count→∞ ⇒ result→mean; count→0 ⇒ result→globalMean.
func Normalize(mean float64, count int, globalMean, confidence float64) float64 {
	c := float64(count)
	return (confidence*globalMean + c*mean) / (confidence + c)
}
