//go:build !openfhe

package main

import (
	"github.com/Fheyalabs/ares-core/pkg/ares/sign"
	"github.com/Fheyalabs/rideshare/internal/ghost"
)

func submitBidReal(d ghost.Driver, inv ghost.Invite, bid int, signer sign.Signer,
	client *ghost.Client, ringDim, depth uint32, sessionID string) {
	_ = d; _ = inv; _ = bid; _ = signer; _ = client; _ = ringDim; _ = depth; _ = sessionID
}
