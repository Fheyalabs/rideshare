package auction

// MaxDrivers is the auction ceiling: rider + ≤5 drivers = ≤6-party N-of-N
// cohort (the benchmarked N=6). Pre-built circuits exist for n=1..MaxDrivers
// so a single willing cab never stalls the rider (spec §5.1).
const MaxDrivers = 5

// DriverBid is one candidate's offer. Only PriceCents is driver-controlled
// and lineage-committed; StarNorm and DistSq are server-authoritative.
type DriverBid struct {
	Pseudonym  string
	PriceCents int     // committed bid, cents
	StarNorm   float64 // server's Bayesian-normalized ★ in [0,5]
	DistSq     float64 // server's coarse pickup-leg, squared
}

// StarPenalty converts the normalized ★ into the key's secondary term:
// higher ★ ⇒ smaller penalty ⇒ lower (better) key.
func (b DriverBid) StarPenalty() float64 { return 5.0 - b.StarNorm }

// WinnerPackage is what the rider's threshold-decrypt reveals (spec §5.5):
// the agreed price (== the winner's committed bid, by mask-select binding),
// the driver pseudonym, and the ★ — never the driver's name.
type WinnerPackage struct {
	WinnerIndex      int
	Pseudonym        string
	AgreedPriceCents int
	StarNorm         float64
}

// SelectCircuitSize clamps a candidate count into the [1, MaxDrivers] family.
func SelectCircuitSize(n int) int {
	if n < 1 {
		return 1
	}
	if n > MaxDrivers {
		return MaxDrivers
	}
	return n
}
