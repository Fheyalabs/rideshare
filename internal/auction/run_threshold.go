//go:build openfhe

package auction

import (
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/helperclient"
)

// PriceBand is the public floor/cap that normalizes a committed bid into the
// [0,1] band before the lexicographic key is assembled. Both bounds are public
// auction parameters (the rider's posted band), so applying them with the
// level-free EvalConstMult leaks nothing the rider doesn't already know.
type PriceBand struct {
	FloorCents int
	CapCents   int
}

// span is the public price range in cents. Guarded against a degenerate band.
func (b PriceBand) span() float64 {
	s := float64(b.CapCents - b.FloorCents)
	if s <= 0 {
		return 1
	}
	return s
}

// scoreScale compresses an assembled lexicographic key into a magnitude where
// every pairwise score difference fed to Argmax stays inside the sharpening
// polynomial's [-1, 1] domain.
//
// The assembled key is  key = K·norm(price) + WStar·starPenalty + WDist·distSq
// with norm(price) ∈ [0,1], starPenalty ∈ [0,5], distSq ≥ 0. The dominant
// (price) term spans [0, K]; the worst-case pairwise key difference is bounded
// by K + WStar·5 + WDist·maxDistSq. We multiply the negated key by this scale
// (level-free) so |score_i − score_j| ≤ 1 and the degree-3 sign indicator is
// evaluated inside its valid range. The scale is a strictly positive constant,
// so it preserves the ranking exactly.
func (w Weights) scoreScale(maxDistSq float64) float64 {
	keySpan := w.K + w.WStar*5.0 + w.WDist*maxDistSq
	if keySpan <= 0 {
		return 1
	}
	// Leave a little headroom (×0.9) so endpoints land strictly inside [-1,1].
	return 0.9 / keySpan
}

// RunAuctionThreshold runs the production real-FHE reverse auction using the
// threshold multi-key protocol. Each driver submits only an encrypted,
// lineage-committed price. The SERVER assembles the lexicographic ranking key
// per driver:
//
//	key_i = K·norm(price_i) + offset_i,  offset_i = WStar·starPenalty_i + WDist·distSq_i
//
// where norm maps the committed price from the public [floor, cap] band into
// [0,1] via the level-free EvalConstMult, and offset_i is injected as an
// ENCRYPTED CONSTANT (EncryptProfile) added with EvalAdd (there is no
// add-const). Argmax over the NEGATED, scaled keys yields one-hot masks; the
// winner package is a mask-select over the RAW committed price ciphertexts
// (Σ EvalMult(mask_i, rawPrice_i)) so the revealed agreed price IS the
// committed bid (binding: "rank cheap, reveal expensive" is impossible).
//
// The agreed price and the masks are threshold-decrypted across the bundle's
// key shares.
func RunAuctionThreshold(
	h *helperclient.Client,
	params helperclient.ContractParams,
	bundle *helperclient.EvalKeyBundle,
	bids []DriverBid,
	w Weights,
	band PriceBand,
) (WinnerPackage, error) {
	n := len(bids)
	if n < 2 {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: need >= 2 bids, got %d", n)
	}
	if bundle == nil || len(bundle.KeyShares) == 0 {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: nil/empty key bundle")
	}

	maxDistSq := 0.0
	for _, b := range bids {
		if b.DistSq > maxDistSq {
			maxDistSq = b.DistSq
		}
	}
	scale := w.scoreScale(maxDistSq)
	span := band.span()
	floor := float64(band.FloorCents)

	rawPrices := make([][]byte, n) // committed bid ciphertexts (the binding payload)
	scores := make([][]byte, n)    // negated, scaled keys → Argmax picks the best

	for i, b := range bids {
		// Encrypt the RAW committed price (cents) — this is the binding payload
		// the mask-select reveals. Slot 0 carries the value; pad to a small
		// batch as the contract expects.
		rawCt, err := h.EncryptProfile(params, bundle.PublicKey, []float64{float64(b.PriceCents), 0, 0, 0})
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: encrypt raw price[%d]: %w", i, err)
		}
		rawPrices[i] = rawCt

		// norm(price) = (price − floor)/span, assembled from the raw price
		// ciphertext with level-free constant ops only:
		//   priceTerm = K·norm(price) = (K/span)·price − (K/span)·floor
		// The additive −(K/span)·floor and the whole offset are folded into one
		// ENCRYPTED CONSTANT injected with EvalAdd (there is no add-const op).
		priceTerm, err := h.EvalConstMult(params, rawCt, w.K/span)
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: scale price[%d]: %w", i, err)
		}

		// Server-authoritative constant for driver i: the ★ / dist² offset
		// minus the price-normalization floor term. This is the only place ★
		// and dist² enter the key, and they enter as a server-signed encrypted
		// constant (drivers cannot influence them).
		offset := w.WStar*b.StarPenalty() + w.WDist*b.DistSq - (w.K/span)*floor
		offsetCt, err := h.EncryptProfile(params, bundle.PublicKey, []float64{offset, 0, 0, 0})
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: encrypt offset[%d]: %w", i, err)
		}

		key, err := h.EvalAdd(params, bundle.EvalKeys, priceTerm, offsetCt)
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: assemble key[%d]: %w", i, err)
		}

		// Negate and compress into the sharpening domain: Argmax picks the
		// MAX, and the best driver has the SMALLEST key, so score = −scale·key.
		// EvalConstMult is level-free, so the negate+scale costs no depth.
		score, err := h.EvalConstMult(params, key, -scale)
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: negate/scale key[%d]: %w", i, err)
		}
		scores[i] = score
	}

	// One-hot masks over the negated, scaled keys. Degree-3 sign indicator
	// mapped to [0,1]: p(x) = 0.5 + 0.75x − 0.25x³ ≈ 1 when x>0, ≈ 0 when x<0.
	masks, err := h.Argmax(params, bundle.EvalKeys, scores, helperclient.ArgmaxParams{
		SharpeningPoly: helperclient.EvalPolyParams{
			Coefficients: []float64{0.5, 0.75, 0, -0.25},
			LowerBound:   -1, UpperBound: 1,
		},
	})
	if err != nil {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: argmax: %w", err)
	}
	if len(masks) != n {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: argmax returned %d masks, want %d", len(masks), n)
	}

	// Mask-select over the RAW committed prices: agreed = Σ_i mask_i · rawPrice_i.
	// The winner's mask is ~1 and losers' ~0, so agreed ≈ the winner's
	// committed bid. Costs one EvalMult level per term plus EvalAdd reductions.
	var agreedCt []byte
	for i := range bids {
		term, err := h.EvalMult(params, bundle.EvalKeys, masks[i], rawPrices[i])
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: mask-select term[%d]: %w", i, err)
		}
		if agreedCt == nil {
			agreedCt = term
			continue
		}
		agreedCt, err = h.EvalAdd(params, bundle.EvalKeys, agreedCt, term)
		if err != nil {
			return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: mask-select add[%d]: %w", i, err)
		}
	}

	// Threshold-decrypt the agreed price and the per-driver masks.
	price, err := thresholdDecryptScalar(h, params, bundle, agreedCt)
	if err != nil {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: decrypt agreed price: %w", err)
	}

	winner, err := decryptWinnerIndex(h, params, bundle, masks)
	if err != nil {
		return WinnerPackage{}, fmt.Errorf("RunAuctionThreshold: decrypt winner index: %w", err)
	}

	return WinnerPackage{
		WinnerIndex:      winner,
		Pseudonym:        bids[winner].Pseudonym,
		AgreedPriceCents: int(price + 0.5),
		StarNorm:         bids[winner].StarNorm,
	}, nil
}

// thresholdDecryptScalar runs the N-of-N partial-decrypt + fuse over every key
// share in the bundle and returns slot 0 of the recovered cleartext.
func thresholdDecryptScalar(
	h *helperclient.Client,
	params helperclient.ContractParams,
	bundle *helperclient.EvalKeyBundle,
	ct []byte,
) (float64, error) {
	partials := make([][]byte, len(bundle.KeyShares))
	for i, share := range bundle.KeyShares {
		p, err := h.PartialDecrypt(params, ct, share.SecretKeyShare, i == 0)
		if err != nil {
			return 0, fmt.Errorf("partial-decrypt share %d: %w", i, err)
		}
		partials[i] = p
	}
	vals, err := h.FusePartials(params, partials, 1)
	if err != nil {
		return 0, fmt.Errorf("fuse partials: %w", err)
	}
	if len(vals) == 0 {
		return 0, fmt.Errorf("fuse partials returned no slots")
	}
	return vals[0], nil
}

// decryptWinnerIndex threshold-decrypts each one-hot mask and returns the index
// of the largest decrypted mask value (the winner). The masks are analog (~1
// for the winner, ~0 for losers), so an argmax over the decrypted values is the
// robust read.
func decryptWinnerIndex(
	h *helperclient.Client,
	params helperclient.ContractParams,
	bundle *helperclient.EvalKeyBundle,
	masks [][]byte,
) (int, error) {
	best, bestVal := -1, 0.0
	for i, m := range masks {
		v, err := thresholdDecryptScalar(h, params, bundle, m)
		if err != nil {
			return 0, fmt.Errorf("decrypt mask %d: %w", i, err)
		}
		if best < 0 || v > bestVal {
			best, bestVal = i, v
		}
	}
	if best < 0 {
		return 0, fmt.Errorf("no masks to decrypt")
	}
	return best, nil
}
