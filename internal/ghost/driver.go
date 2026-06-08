// Package ghost implements ghost driver behaviour (movement, threshold,
// bidding) and a thin test rider. Ghosts hit the same HTTP API as real phones.
package ghost

// Invite mirrors the server's Invite (avoiding an import cycle).
type Invite struct {
	SessionID    string `json:"session_id"`
	OfferedPrice int    `json:"offered_price"`
	DropoffHex   string `json:"dropoff_hex"`
	PKHandle     string `json:"pk_handle"`
}

// PriceStrategy computes the bid amount from the rider's offered price.
type PriceStrategy func(offer int) int

// PercentBelow returns a strategy that bids `(1-pct)` of the offered price.
func PercentBelow(pct float64) PriceStrategy {
	return func(offer int) int {
		return int(float64(offer) * (1 - pct))
	}
}

// Fixed returns a strategy that always bids the same amount.
func Fixed(cents int) PriceStrategy {
	return func(int) int { return cents }
}

// Driver is a ghost driver agent.
type Driver struct {
	Pseudonym string
	MinCents  int
	Strategy  PriceStrategy
}

// DecideBid returns (bidCents, true) if the offer meets the driver's
// min-€ threshold, or (0, false) if the driver declines.
func (d Driver) DecideBid(inv Invite) (int, bool) {
	if inv.OfferedPrice < d.MinCents {
		return 0, false
	}
	bid := d.Strategy(inv.OfferedPrice)
	if bid < 0 {
		bid = 0
	}
	return bid, true
}
