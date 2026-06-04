package fhe

import (
	"math"
	"testing"
)

// keysByN are the representative per-cohort key vectors shared by the plaintext
// design test and the openfhe calibration test. Each value is a normalized key
// in [0,1]; the minimum index (the expected one-hot slot) is distinct in every
// case.
var keysByN = map[int][]float64{
	2: {0.20, 0.80},
	3: {0.50, 0.20, 0.80},
	4: {0.60, 0.20, 0.40, 0.80},
	5: {0.70, 0.20, 0.55, 0.40, 0.90},
}

// minItersByN is the minimum sign-composition count k at which the circuit's
// analog mask is within tolerance 0.15 of the exact one-hot, per cohort size.
// These drive both the plaintext design check and the calibration test's
// per-n configuration. Verified by TestPlaintextMaskWithinTolerance and matched
// against the real-FHE measurement in CALIBRATION.md.
var minItersByN = map[int]int{2: 1, 3: 3, 4: 4, 5: 4}

// TestExpectedIsExactOneHot verifies the ground-truth oracle is the exact
// one-hot of the minimum key, independent of any FHE machinery.
func TestExpectedIsExactOneHot(t *testing.T) {
	for n := 2; n <= 5; n++ {
		cut := ArgminCircuit{Keys: keysByN[n]}
		got := cut.Expected(cut.Inputs())
		minIdx, ones := 0, 0
		for i, v := range got {
			if v == 1.0 {
				ones++
			}
			if keysByN[n][i] < keysByN[n][minIdx] {
				minIdx = i
			}
		}
		if ones != 1 || got[minIdx] != 1.0 {
			t.Fatalf("n=%d: Expected not a clean one-hot of min idx %d: %v", n, minIdx, got)
		}
	}
}

// TestPlaintextMaskWithinTolerance asserts that the circuit DESIGN (the product
// of composed-sign step polynomials) reproduces the exact one-hot within the
// calibration tolerance at the configured minItersByN -- a noise-free
// correctness check of the mask construction before FHE is involved.
func TestPlaintextMaskWithinTolerance(t *testing.T) {
	const tol = 0.15
	for n := 2; n <= 5; n++ {
		k := minItersByN[n]
		mask := plaintextMask(keysByN[n], 3, k)
		want := ArgminCircuit{Keys: keysByN[n]}.Expected([][]float64{keysByN[n]})
		worst := 0.0
		for i := range mask {
			if d := math.Abs(mask[i] - want[i]); d > worst {
				worst = d
			}
		}
		t.Logf("n=%d iters=%d worstAbsErr=%.4f mask=%v", n, k, worst, mask)
		if worst > tol {
			t.Fatalf("n=%d iters=%d: plaintext mask worst err %.4f exceeds tol %.2f",
				n, k, worst, tol)
		}
		// argmax of the analog mask must be the true minimum slot.
		best := 0
		for i := 1; i < len(mask); i++ {
			if mask[i] > mask[best] {
				best = i
			}
		}
		if want[best] != 1.0 {
			t.Fatalf("n=%d: mask argmax slot %d is not the true minimum", n, best)
		}
	}
}

// TestStepMonotoneAtZero pins the self-comparison value step(0)=0.5 used by the
// homomorphic self-factor neutralization (mask.go) for every iteration count.
func TestStepMonotoneAtZero(t *testing.T) {
	for k := 1; k <= 4; k++ {
		if got := plaintextStep(3, k, 0.0); math.Abs(got-0.5) > 1e-12 {
			t.Fatalf("iters=%d: step(0)=%v, want 0.5", k, got)
		}
		// step should be increasing across 0 (negative diff -> ~0, positive -> ~1).
		if plaintextStep(3, k, -0.5) >= plaintextStep(3, k, 0.5) {
			t.Fatalf("iters=%d: step not increasing across 0", k)
		}
	}
}
