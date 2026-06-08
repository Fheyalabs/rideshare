//go:build openfhe

package ghost

import (
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/auction"
)

// EncryptSignSubmit encrypts the driver's bid under the rider's pk, signs it,
// uploads the ciphertext, and submits the signed bid to the session.
func (d *Driver) EncryptSignSubmit(
	params cgo.ContractParams,
	sessionID string,
	inv Invite,
	bidCents int,
	signer sign.Signer,
	client *Client,
) error {
	// Fetch rider's public key.
	pk, err := client.GetArtifact(inv.PKHandle)
	if err != nil {
		return fmt.Errorf("%s: fetch pk: %w", d.Pseudonym, err)
	}
	// Encrypt bid.
	enc, err := cgo.SingleKeyEncrypt(params, pk, float64(bidCents))
	if err != nil {
		return fmt.Errorf("%s: encrypt bid: %w", d.Pseudonym, err)
	}
	// Upload encrypted bid ciphertext.
	h, err := client.PutArtifact(enc)
	if err != nil {
		return fmt.Errorf("%s: put artifact: %w", d.Pseudonym, err)
	}
	// Sign.
	nonce := []byte(d.Pseudonym)
	sig, err := auction.SignBid(signer, []byte(sessionID), enc, nonce)
	if err != nil {
		return fmt.Errorf("%s: sign bid: %w", d.Pseudonym, err)
	}
	// Submit.
	return client.SubmitBid(sessionID, h, nonce, signer.PublicKey(), sig)
}
