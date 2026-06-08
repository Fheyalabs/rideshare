//go:build openfhe

package ghost

import (
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
)

// Rider is a thin test rider: keygens, opens a session, fetches masks, decrypts.
type Rider struct {
	Params cgo.ContractParams
	SK     []byte
	PK     []byte
	Client *Client
}

// Keygen generates a single-key CKKS keypair.
func (r *Rider) Keygen() error {
	pk, sk, err := cgo.SingleKeyGen(r.Params)
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}
	r.PK = pk
	r.SK = sk
	return nil
}

// OpenSession uploads the pk and opens a ride session.
func (r *Rider) OpenSession(req map[string]any) ([]string, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("no client")
	}
	h, err := r.Client.PutArtifact(r.PK)
	if err != nil {
		return nil, fmt.Errorf("upload pk: %w", err)
	}
	req["pk_handle"] = h
	return r.Client.OpenSession(req)
}

// Winner decrypts the mask ciphertexts and returns the winner index + mask values.
func (r *Rider) Winner(sessionID string) (int, []float64, error) {
	handles, err := r.Client.GetMasks(sessionID)
	if err != nil {
		return -1, nil, fmt.Errorf("get masks: %w", err)
	}
	masks := make([]float64, len(handles))
	best, bestVal := -1, 0.0
	for i, h := range handles {
		ct, err := r.Client.GetArtifact(h)
		if err != nil {
			return -1, nil, fmt.Errorf("get mask[%d]: %w", i, err)
		}
		vals, err := cgo.SingleKeyDecrypt(r.Params, r.SK, ct, 1)
		if err != nil {
			return -1, nil, fmt.Errorf("decrypt mask[%d]: %w", i, err)
		}
		masks[i] = vals[0]
		if best < 0 || masks[i] > bestVal {
			best, bestVal = i, masks[i]
		}
	}
	return best, masks, nil
}
