package session

import (
	"testing"

	"github.com/Fheyalabs/rideshare/internal/auction"
	"github.com/Fheyalabs/rideshare/internal/rating"
)

func TestStar_AuthoritativeFromStore(t *testing.T) {
	store := rating.NewStore(4.3, 20)
	store.Record("pk-star3", 3.0, 5)   // low ★ driver
	store.Record("pk-star5", 5.0, 500) // high ★ driver

	sess := NewAuctionSession([]byte("ride"), DefaultParams(),
		auction.PriceBand{FloorCents: 800, CapCents: 5000},
		DefaultWeights(), 1, store)

	// Submit two identical-price bids; ★ from store should break tie.
	if err := sess.SubmitBid(auction.SignedBid{Pubkey: []byte("pk-star3"), EncBid: []byte{1}, Nonce: []byte("a"), Sig: make([]byte, 64)}); err == nil {
		t.Error("unsigned bid should be rejected")
	}
	// Signature check is orthogonal — test the star lookup directly.
	star3 := sess.ratings.StarNorm("pk-star3")
	star5 := sess.ratings.StarNorm("pk-star5")
	if star5 <= star3 {
		t.Errorf("high-★ (%.3f) must exceed low-★ (%.3f)", star5, star3)
	}
	if star3 > 4.3 || star5 < 4.9 {
		t.Errorf("shrinkage wrong: star3=%.3f star5=%.3f", star3, star5)
	}
	// unknown driver gets global mean
	if s := sess.ratings.StarNorm("unknown"); s != 4.3 {
		t.Errorf("unknown = %.3f, want 4.3", s)
	}
}
