// SPDX-License-Identifier: Apache-2.0

//go:build openfhe

package fhe

import (
	"os"
	"testing"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/cgo"
	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/fhecalib"
	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/helperclient"
)

// calibCase pins, per cohort size, the calibrated configuration and the
// measured/predicted minimum converging CKKS multiplicative depth. See
// CALIBRATION.md for how these were obtained and why n>=3 is RAM-gated.
type calibCase struct {
	n        int
	iters    int    // sign-composition count (minItersByN)
	ringDim  uint32 // ring that admits the required depth at this machine
	minDepth uint32 // measured (n=2) or predicted (n>=3) min converging depth
	bigRing  bool   // requires ring 2^16 keygen (~6-9 GB RAM): gated
}

// argminCalibCases is the calibration matrix. n=2 is measured end-to-end on
// real FHE in every run; n>=3 needs ring 2^16 / depth >=12, whose threshold
// keygen exceeds ~6 GB RAM, so it runs only under ARGMIN_BIG_RING=1.
var argminCalibCases = []calibCase{
	{n: 2, iters: 1, ringDim: 1 << 15, minDepth: 5, bigRing: false},
	{n: 3, iters: 3, ringDim: 1 << 16, minDepth: 12, bigRing: true},
	{n: 4, iters: 4, ringDim: 1 << 16, minDepth: 15, bigRing: true},
	{n: 5, iters: 4, ringDim: 1 << 16, minDepth: 16, bigRing: true},
}

// TestCalibrate_ArgminDepthWithinBudget MEASURES, against the real OpenFHE
// backend, the minimum secure CKKS multiplicative depth at which the
// homomorphic argmin's decrypted one-hot matches the plaintext argmin
// (tolerance 0.15 per slot).
//
// Each case sweeps depth from the recorded minDepth (the structural floor;
// below it the circuit hard-errors on exhausted levels rather than missing
// tolerance) up to a small ceiling, and asserts the calibrator converges AT
// that depth -- i.e. minDepth is both necessary and sufficient.
//
// The spec's "depth <= 8" was a HYPOTHESIS; the measured minimum for this
// ContextHandle primitive set is n-dependent and exceeds 8 for n>=3. See
// CALIBRATION.md.
func TestCalibrate_ArgminDepthWithinBudget(t *testing.T) {
	if err := cgo.SmokeCKKS(); err != nil {
		t.Skipf("OpenFHE unavailable: %v", err)
	}
	bigRing := os.Getenv("ARGMIN_BIG_RING") == "1"

	for _, tc := range argminCalibCases {
		tc := tc
		t.Run(name(tc.n), func(t *testing.T) {
			if tc.bigRing && !bigRing {
				t.Skipf("n=%d needs ring %d / depth %d (~6-9 GB RAM threshold keygen); "+
					"set ARGMIN_BIG_RING=1 to run. Predicted min depth %d (iters=%d). See CALIBRATION.md.",
					tc.n, tc.ringDim, tc.minDepth, tc.minDepth, tc.iters)
			}

			cut := ArgminCircuit{Keys: keysByN[tc.n], SignDegree: 3, SignIters: tc.iters}
			res, err := fhecalib.Calibrate(cut, fhecalib.CalibrationParams{
				Base: helperclient.ContractParams{
					RingDim:        tc.ringDim,
					ScalingModSize: 50,
				},
				StartDepth: tc.minDepth,
				MaxDepth:   tc.minDepth + 2,
				Tolerance:  0.15,
			}, tc.n /* profileDim = slot count */)
			if err != nil {
				t.Fatalf("n=%d calibrate: %v", tc.n, err)
			}

			t.Logf("n=%d iters=%d ring=%d -> depth=%d passed=%v bestAbsErr=%.4f",
				tc.n, tc.iters, res.RingDim, res.Depth, res.Passed, res.AchievedAbsError)

			if !res.Passed {
				t.Fatalf("n=%d: did not converge by depth %d (best abs err %.4f)",
					tc.n, tc.minDepth+2, res.AchievedAbsError)
			}
			if res.Depth != tc.minDepth {
				t.Fatalf("n=%d: converged at depth %d, expected recorded min depth %d",
					tc.n, res.Depth, tc.minDepth)
			}
		})
	}
}

func name(n int) string {
	switch n {
	case 2:
		return "n=2"
	case 3:
		return "n=3"
	case 4:
		return "n=4"
	default:
		return "n=5"
	}
}
