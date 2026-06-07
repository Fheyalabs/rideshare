//go:build openfhe

package session

import (
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/auction"
)

func TestSession_BlindAuction_CheapestSignedWins(t *testing.T) {
	params := cgo.ContractParams{
		RingDim: 1 << 15, Depth: 5,
		ScalingFactor: float64(uint64(1) << 50),
	}
	pk, sk, err := cgo.SingleKeyGen(params)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	sess := NewAuctionSession([]byte("ride-1"), params,
		auction.PriceBand{FloorCents: 800, CapCents: 5000},
		cgo.AuctionWeights{K: 100, WStar: 1, WDist: 0.001}, 1)
	sess.SetRiderPK(pk)

	registered := map[string]bool{}
	drivers := []struct {
		name         string
		price        int
		star, distSq float64
	}{
		{"drv-A", 1290, 4.8, 1.0},
		{"drv-B", 1190, 4.1, 9.0},
		{"drv-C", 1240, 4.5, 4.0},
	}
	for _, d := range drivers {
		signer, _ := sign.NewEd25519Signer()
		registered[string(signer.PublicKey())] = true
		enc, _ := cgo.SingleKeyEncrypt(params, pk, float64(d.price))
		nonce := []byte(d.name)
		sig, _ := auction.SignBid(signer, []byte("ride-1"), enc, nonce)
		if err := sess.SubmitBid(auction.SignedBid{
			EncBid: enc, Nonce: nonce,
			Pubkey: signer.PublicKey(), Sig: sig,
		}, d.star, d.distSq); err != nil {
			t.Fatalf("submit %s: %v", d.name, err)
		}
	}

	masks, err := sess.RunBlindAuction()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Test-side rider decrypt:
	_, winner, err := cgo.SingleKeyAuctionDecrypt(params, sk, masks)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if got := sess.PseudonymAt(winner); len(got) == 0 {
		t.Fatalf("no pseudonym at winner index %d", winner)
	}
	// Cheapest is drv-B at €11.90
	if winner != 1 {
		t.Errorf("cheapest signed bidder (drv-B €11.90) must win, got idx=%d", winner)
	}

	// Verify winner signature
	winBid := sess.bids[winner].sb
	if err := auction.VerifyWinner([]byte("ride-1"), winBid, registered); err != nil {
		t.Errorf("winner signature verification failed: %v", err)
	}
	t.Logf("blind auction OK: winner=%d masks=%d", winner, len(masks))
}

func TestSession_RejectsUnsignedBid(t *testing.T) {
	params := cgo.ContractParams{
		RingDim: 1 << 14, Depth: 4,
		ScalingFactor: float64(uint64(1) << 50),
	}
	pk, _, _ := cgo.SingleKeyGen(params)
	sess := NewAuctionSession([]byte("ride-2"), params,
		auction.PriceBand{FloorCents: 800, CapCents: 5000},
		cgo.AuctionWeights{K: 100, WStar: 1, WDist: 0.001}, 1)
	sess.SetRiderPK(pk)

	// Submit a bid with a WRONG signature — server forges sig with different key
	realSigner, _ := sign.NewEd25519Signer()
	wrongSigner, _ := sign.NewEd25519Signer()
	enc, _ := cgo.SingleKeyEncrypt(params, pk, 1000)
	nonce := []byte("ghost")
	// Sign with realSigner, but submit with wrongSigner's pubkey
	sig, _ := auction.SignBid(realSigner, []byte("ride-2"), enc, nonce)
	err := sess.SubmitBid(auction.SignedBid{
		EncBid: enc,
		Nonce:  nonce,
		Pubkey: wrongSigner.PublicKey(), // wrong pubkey!
		Sig:    sig,                      // sig from a different key
	}, 5.0, 0.1)
	if err == nil {
		t.Fatal("bid with wrong-pubkey signature should be rejected at submit time")
	}
	t.Logf("wrong-sig bid correctly rejected: %v", err)
}
