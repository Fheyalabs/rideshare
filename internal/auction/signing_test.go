package auction

import (
	"bytes"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
)

var (
	testSessionID = []byte("ride-1")
	testEncBid    = []byte("ciphertext-bytes")
	testNonce     = []byte("nonce-0")
)

func newTestSigner(t *testing.T) *sign.Ed25519Signer {
	t.Helper()
	s, err := sign.NewEd25519Signer()
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	return s
}

// TestSignBid_VerifyRoundTrip checks that a fresh signer can sign a bid and
// the resulting SignedBid verifies cleanly under the same session.
func TestSignBid_VerifyRoundTrip(t *testing.T) {
	signer := newTestSigner(t)

	sig, err := SignBid(signer, testSessionID, testEncBid, testNonce)
	if err != nil {
		t.Fatalf("SignBid: %v", err)
	}

	sb := SignedBid{
		EncBid: testEncBid,
		Nonce:  testNonce,
		Pubkey: signer.PublicKey(),
		Sig:    sig,
	}

	if err := sb.Verify(testSessionID); err != nil {
		t.Fatalf("Verify returned unexpected error: %v", err)
	}
}

// TestSignedBid_RejectsTampered ensures that flipping any byte in EncBid,
// Nonce, or Sig causes Verify to return a non-nil error.
func TestSignedBid_RejectsTampered(t *testing.T) {
	signer := newTestSigner(t)

	sig, err := SignBid(signer, testSessionID, testEncBid, testNonce)
	if err != nil {
		t.Fatalf("SignBid: %v", err)
	}

	goodSB := SignedBid{
		EncBid: testEncBid,
		Nonce:  testNonce,
		Pubkey: signer.PublicKey(),
		Sig:    sig,
	}

	// Sanity-check the baseline is valid.
	if err := goodSB.Verify(testSessionID); err != nil {
		t.Fatalf("baseline Verify failed: %v", err)
	}

	flipByte := func(src []byte) []byte {
		cp := bytes.Clone(src)
		cp[0] ^= 0xFF
		return cp
	}

	t.Run("tampered_EncBid", func(t *testing.T) {
		sb := SignedBid{
			EncBid: flipByte(goodSB.EncBid),
			Nonce:  goodSB.Nonce,
			Pubkey: goodSB.Pubkey,
			Sig:    goodSB.Sig,
		}
		if err := sb.Verify(testSessionID); err == nil {
			t.Fatal("expected Verify to fail on tampered EncBid, got nil")
		}
	})

	t.Run("tampered_Nonce", func(t *testing.T) {
		sb := SignedBid{
			EncBid: goodSB.EncBid,
			Nonce:  flipByte(goodSB.Nonce),
			Pubkey: goodSB.Pubkey,
			Sig:    goodSB.Sig,
		}
		if err := sb.Verify(testSessionID); err == nil {
			t.Fatal("expected Verify to fail on tampered Nonce, got nil")
		}
	})

	t.Run("tampered_Sig", func(t *testing.T) {
		sb := SignedBid{
			EncBid: goodSB.EncBid,
			Nonce:  goodSB.Nonce,
			Pubkey: goodSB.Pubkey,
			Sig:    flipByte(goodSB.Sig),
		}
		if err := sb.Verify(testSessionID); err == nil {
			t.Fatal("expected Verify to fail on tampered Sig, got nil")
		}
	})
}

// TestVerifyWinner_RejectsUnregisteredKey ensures a valid signature from a
// driver whose pubkey is not in the registered set is rejected (ghost-driver
// detection), and that a registered + valid driver is accepted.
func TestVerifyWinner_RejectsUnregisteredKey(t *testing.T) {
	signer := newTestSigner(t)

	sig, err := SignBid(signer, testSessionID, testEncBid, testNonce)
	if err != nil {
		t.Fatalf("SignBid: %v", err)
	}

	winner := SignedBid{
		EncBid: testEncBid,
		Nonce:  testNonce,
		Pubkey: signer.PublicKey(),
		Sig:    sig,
	}

	t.Run("unregistered_ghost_driver", func(t *testing.T) {
		emptyReg := RegisteredSet() // no keys registered
		err := VerifyWinner(testSessionID, winner, emptyReg)
		if err == nil {
			t.Fatal("expected VerifyWinner to reject unregistered pubkey, got nil")
		}
		if want := "ghost driver"; !bytes.Contains([]byte(err.Error()), []byte(want)) {
			t.Fatalf("expected error to mention %q, got: %v", want, err)
		}
	})

	t.Run("registered_valid_driver", func(t *testing.T) {
		reg := RegisteredSet(signer.PublicKey())
		if err := VerifyWinner(testSessionID, winner, reg); err != nil {
			t.Fatalf("expected VerifyWinner to accept registered+valid driver, got: %v", err)
		}
	})
}

// TestVerifyWinner_RejectsWrongSessionID verifies that a bid signed under
// session A is rejected when verified under session B (session binding).
func TestVerifyWinner_RejectsWrongSessionID(t *testing.T) {
	signer := newTestSigner(t)
	sessionA := []byte("ride-A")
	sessionB := []byte("ride-B")

	sig, err := SignBid(signer, sessionA, testEncBid, testNonce)
	if err != nil {
		t.Fatalf("SignBid: %v", err)
	}

	winner := SignedBid{
		EncBid: testEncBid,
		Nonce:  testNonce,
		Pubkey: signer.PublicKey(),
		Sig:    sig,
	}

	reg := RegisteredSet(signer.PublicKey())

	// Verifying under the correct session must succeed.
	if err := VerifyWinner(sessionA, winner, reg); err != nil {
		t.Fatalf("VerifyWinner with correct session failed: %v", err)
	}

	// Verifying under a different session must fail.
	if err := VerifyWinner(sessionB, winner, reg); err == nil {
		t.Fatal("expected VerifyWinner to fail when session ID differs, got nil")
	}
}
