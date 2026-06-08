//go:build openfhe

package main

import (
	"log"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/ghost"
)

func submitBidReal(d ghost.Driver, inv ghost.Invite, bid int, signer sign.Signer,
	client *ghost.Client, ringDim, depth uint32, sessionID string) {
	params := cgo.ContractParams{RingDim: ringDim, Depth: depth, ScalingFactor: float64(uint64(1) << 50)}
	if err := d.EncryptSignSubmit(params, sessionID, inv, bid, signer, client); err != nil {
		log.Printf("[%s] bid submit failed: %v", d.Pseudonym, err)
	}
}
