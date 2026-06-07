//go:build openfhe

package session

import (
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/auction"
	"github.com/Fheyalabs/rideshare/internal/rating"
)

func TestStar_BreaksPriceTie(t *testing.T) {
	cgoParams := cgo.ContractParams{RingDim: 1 << 15, Depth: 5, ScalingFactor: float64(uint64(1) << 50)}
	pk, sk, err := cgo.SingleKeyGen(cgoParams)
	if err != nil { t.Fatalf("keygen: %v", err) }

	// Seed ratings: drv-A 5★ (many votes), drv-B 3★ (few votes).
	// Equal prices → ★ should break tie → drv-A wins.
	store := rating.NewStore(4.3, 20)
	signerA, _ := sign.NewEd25519Signer()
	signerB, _ := sign.NewEd25519Signer()
	store.Record(string(signerA.PublicKey()), 5.0, 500)
	store.Record(string(signerB.PublicKey()), 3.0, 5)

	params := DefaultParams()
	sess := NewAuctionSession([]byte("ride-star"), params,
		auction.PriceBand{FloorCents: 800, CapCents: 5000},
		DefaultWeights(), 1, store)
	sess.SetRiderPK(pk)

	// Two identical-price bids: ★ should break the tie.
	for _, d := range []struct {
		signer sign.Signer; price int; name []byte
	}{
		{signerA, 1200, []byte("drv-A")},
		{signerB, 1200, []byte("drv-B")},
	} {
		enc, _ := cgo.SingleKeyEncrypt(cgoParams, pk, float64(d.price))
		sig, _ := auction.SignBid(d.signer, []byte("ride-star"), enc, d.name)
		_ = sess.SubmitBid(auction.SignedBid{
			EncBid: enc, Nonce: d.name, Pubkey: d.signer.PublicKey(), Sig: sig,
		})
	}

	masks, err := sess.RunBlindAuction()
	if err != nil { t.Fatalf("run: %v", err) }

	_, winner, err := cgo.SingleKeyAuctionDecrypt(cgoParams, sk, masks)
	if err != nil { t.Fatalf("decrypt: %v", err) }

	if winner != 0 {
		t.Errorf("equal price + higher ★: expected drv-A (idx 0, 5★) to win, got idx=%d", winner)
	}
	t.Logf("★ broke price tie: winner=%d (expected 0, better ★)", winner)
}
