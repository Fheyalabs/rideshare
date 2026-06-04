package auction

import (
	"crypto/sha256"
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
)

// SignedBid is a driver's submission: the encrypted bid ciphertext, a fresh
// per-ride nonce, the driver's Ed25519 public key, and a signature over
// H(sessionID || encBid || nonce). The rider verifies the WINNING driver's
// SignedBid so a server-spawned ghost driver cannot win undetected.
type SignedBid struct {
	EncBid []byte
	Nonce  []byte
	Pubkey []byte
	Sig    []byte
}

// BidDigest is the message a driver signs: SHA256(sessionID || encBid || nonce).
func BidDigest(sessionID, encBid, nonce []byte) []byte {
	h := sha256.New()
	h.Write(sessionID)
	h.Write(encBid)
	h.Write(nonce)
	return h.Sum(nil)
}

// SignBid produces a driver's signature over its bid digest.
func SignBid(signer sign.Signer, sessionID, encBid, nonce []byte) ([]byte, error) {
	return signer.Sign(BidDigest(sessionID, encBid, nonce))
}

// Verify checks the signature over this bid's digest under its own pubkey.
func (sb SignedBid) Verify(sessionID []byte) error {
	v, err := sign.NewEd25519Signer()
	if err != nil {
		return fmt.Errorf("verifier init: %w", err)
	}
	return v.Verify(sb.Pubkey, BidDigest(sessionID, sb.EncBid, sb.Nonce), sb.Sig)
}

// VerifyWinner verifies the winning driver's SignedBid: the signature must be
// valid AND the driver's pubkey must be in the registered set (a ghost driver
// the server spawned has no registered identity key). registered maps the raw
// pubkey bytes (as a string key) to true.
func VerifyWinner(sessionID []byte, winner SignedBid, registered map[string]bool) error {
	if !registered[string(winner.Pubkey)] {
		return fmt.Errorf("winner pubkey not registered (possible ghost driver)")
	}
	if err := winner.Verify(sessionID); err != nil {
		return fmt.Errorf("winner signature invalid: %w", err)
	}
	return nil
}

// RegisteredSet builds a map[string]bool from a list of raw pubkey byte slices.
// Callers use it to construct the set passed to VerifyWinner.
func RegisteredSet(pubkeys ...[]byte) map[string]bool {
	m := make(map[string]bool, len(pubkeys))
	for _, pk := range pubkeys {
		m[string(pk)] = true
	}
	return m
}
