//go:build openfhe

package auction

import (
	"context"
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/helperclient"
)

// TestStep0_HelperOpsImplemented is the gate from the task plan: confirm the
// C++ helper actually implements eval_const_mult / eval_add / argmax (the doc
// comment in scoring_ops.go says "planned, ErrNotImplemented" but the dispatch
// in cmd/openfhe-contract-helper/main.go routes to real cgo bridge calls). If
// any op returns ErrNotImplemented (or an "unsupported op" / "not implemented"
// error), RunAuction is BLOCKED.
func TestStep0_HelperOpsImplemented(t *testing.T) {
	bin := os.Getenv("ARES_HELPER_BINARY")
	if bin == "" {
		t.Skip("set ARES_HELPER_BINARY")
	}
	c, err := helperclient.Start(context.Background(), bin)
	if err != nil {
		t.Fatalf("start helper: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	params := helperclient.ContractParams{RingDim: 1 << 15, Depth: 6, ScalingModSize: 50}
	bundle, err := c.KeygenChain(params, 2)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}

	a, err := c.EncryptProfile(params, bundle.PublicKey, []float64{0.5, 0, 0, 0})
	if err != nil {
		t.Fatalf("encrypt a: %v", err)
	}
	b, err := c.EncryptProfile(params, bundle.PublicKey, []float64{-0.3, 0, 0, 0})
	if err != nil {
		t.Fatalf("encrypt b: %v", err)
	}

	if _, err := c.EvalConstMult(params, a, 2.0); err != nil {
		t.Fatalf("EvalConstMult: %v", err)
	}
	if _, err := c.EvalAdd(params, bundle.EvalKeys, a, b); err != nil {
		t.Fatalf("EvalAdd: %v", err)
	}
	masks, err := c.Argmax(params, bundle.EvalKeys, [][]byte{a, b}, helperclient.ArgmaxParams{
		// degree-3 sign mapped to a [0,1] indicator: 0.5 + 0.75x - 0.25x^3.
		SharpeningPoly: helperclient.EvalPolyParams{
			Coefficients: []float64{0.5, 0.75, 0, -0.25},
			LowerBound:   -1, UpperBound: 1,
		},
	})
	if err != nil {
		t.Fatalf("Argmax: %v", err)
	}
	if len(masks) != 2 {
		t.Fatalf("argmax returned %d masks, want 2", len(masks))
	}
	t.Logf("STEP 0 OK: EvalConstMult, EvalAdd, Argmax all return real ciphertexts (no ErrNotImplemented)")
}
