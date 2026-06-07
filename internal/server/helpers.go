package server

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/Fheyalabs/rideshare/internal/auction"
	"github.com/Fheyalabs/rideshare/internal/session"
)

const holdTTL = 60 * time.Second

func hexEncode(b []byte) string    { return hex.EncodeToString(b) }
func hexDecode(s string, out []byte) error {
	d, err := hex.DecodeString(s)
	if err != nil { return err }
	if len(d) != len(out) { return fmt.Errorf("hex decode: expected %d bytes, got %d", len(out), len(d)) }
	copy(out, d)
	return nil
}

func b64Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
func itoa(i int) string                  { return strconv.Itoa(i) }

func auctionPriceBand(floor, cap int) auction.PriceBand {
	return auction.PriceBand{FloorCents: floor, CapCents: cap}
}

type auctionSignedBid = auction.SignedBid

func defaultSessionParams(ringDim, depth uint32) session.ContractParams {
	return session.ContractParams{RingDim: ringDim, Depth: depth, ScalingFactor: float64(uint64(1) << 50)}
}
