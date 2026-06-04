// Package fhe defines the rideshare cohort's price-only homomorphic argmin
// circuit as an ares-core fhecalib.CircuitUnderTest, so its minimum secure
// CKKS multiplicative depth can be MEASURED (not assumed) against the real
// OpenFHE backend.
//
// The circuit is backend-agnostic: it is expressed purely against the
// fhecalib.ContextHandle primitive surface (EvalMult / EvalSubConst), so it
// compiles without the openfhe build tag. Only the calibration *test* and the
// homomorphic mask construction that drives a real ContextHandle are gated.
package fhe

// ArgminCircuit is the price-only argmin over a cohort of n normalized keys
// packed into slots 0..n-1.
//
// Definition. Given encrypted keys k = [k_0, ..., k_{n-1}] with every k_i in
// [0,1], the circuit emits the packed one-hot mask of the MINIMUM key: slot i
// is ~1 for the single smallest key and ~0 elsewhere. Slot i is
//
//	mask_i = Π_{j != i} step(k_j - k_i)
//
// where step(x) ~ 1 for x > 0 (k_j is larger, so i is the smaller of the pair)
// and step(x) ~ 0 for x < 0. The product is ~1 only for the global minimum and
// decays to ~0 for any candidate that loses at least one pairwise comparison.
//
// step is built by COMPOSING an odd-degree sign-approximation polynomial g
// with itself SignIters times -- g^(k)(x) sharpens toward true sign(x) on
// [-1,1] -- then mapping the [-1,1] sign output to a [0,1] indicator via
// step(x) = (g^(k)(x)+1)/2. Composition (not a single high-degree polynomial)
// is forced by the ContextHandle surface: with no ciphertext+ciphertext
// addition available, a polynomial can only be evaluated in Horner form, whose
// multiplicative depth equals its degree -- so a degree-3 g applied k times
// (depth ~3k) is far cheaper than one degree-3^k polynomial.
type ArgminCircuit struct {
	// Keys are the cohort's normalized keys in [0,1], one per slot 0..n-1.
	// The plaintext values are known to the circuit definition; the
	// calibrator encrypts them before Eval runs.
	Keys []float64
	// SignDegree selects the per-iteration sign-approximation polynomial g:
	// 3 (cubic 1.5x-0.5x^3) or 9 (degree-9 minimax). 3 is the depth-cheapest
	// per application (3 multiplicative levels) and is the calibrated choice.
	SignDegree int
	// SignIters is the self-composition count k of g. Higher k sharpens the
	// transition band toward a true step (so larger cohorts separate into a
	// clean one-hot) at a cost of ~ (deg-1)... levels per iteration. Default 1.
	SignIters int
}

func (c ArgminCircuit) Name() string { return "price-argmin-onehot" }

// iters returns the effective composition count (at least 1).
func (c ArgminCircuit) iters() int {
	if c.SignIters < 1 {
		return 1
	}
	return c.SignIters
}

// Inputs returns the encrypted-input layout the homomorphic Eval consumes.
//
// Row 0 is the packed key vector k. The remaining rows are public CONSTANT
// vectors that Eval needs as ciphertexts: the fhecalib.ContextHandle exposes
// no scalar-multiply, so every scalar coefficient of the sign polynomial is
// supplied as an encrypted constant and applied via EvalMult. See mask.go
// (in* indices) for the layout contract. The non-openfhe build still needs a
// consistent Inputs() for Expected()/Name() callers, so the layout is defined
// here unconditionally.
func (c ArgminCircuit) Inputs() [][]float64 {
	n := len(c.Keys)
	keyRow := make([]float64, n)
	copy(keyRow, c.Keys)
	zeros := make([]float64, n)
	seed := fill(n, signCoeffLead(c.SignDegree)) // leading Horner seed scalar
	return [][]float64{keyRow, zeros, seed}
}

// Expected returns the exact plaintext one-hot of the minimum key.
//
// Ties resolve to the lowest index (matching the strict ">" the homomorphic
// step uses for j-vs-i comparisons across distinct indices). Inputs[0] is the
// key row; constant rows are ignored.
func (c ArgminCircuit) Expected(inputs [][]float64) []float64 {
	keys := inputs[0]
	out := make([]float64, len(keys))
	if len(keys) == 0 {
		return out
	}
	minIdx := 0
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[minIdx] {
			minIdx = i
		}
	}
	out[minIdx] = 1.0
	return out
}

// fill returns an n-length slice with every slot set to v.
func fill(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// signCoeffLead returns the leading (highest-degree) coefficient of the
// per-iteration sign polynomial g, supplied as the encrypted Horner seed.
func signCoeffLead(degree int) float64 {
	cs := signCoeffs(degree)
	return cs[len(cs)-1]
}

// signCoeffs returns the ascending-order coefficients of the per-iteration sign
// approximation g(x) ~ sign(x) on [-1,1]. Shared by the plaintext reference and
// the homomorphic Horner evaluation so both stay in lockstep.
//
// degree 3: g(x) = 1.5x - 0.5x^3
// degree 9: minimax degree-9 odd sign approximation
func signCoeffs(degree int) []float64 {
	switch degree {
	case 9:
		// g9(x) = (315/128)x - (105/32)x^3 + (189/64)x^5 - (45/32)x^7 + (35/128)x^9
		return []float64{
			0,
			315.0 / 128.0,
			0,
			-105.0 / 32.0,
			0,
			189.0 / 64.0,
			0,
			-45.0 / 32.0,
			0,
			35.0 / 128.0,
		}
	default: // 3
		return []float64{0, 1.5, 0, -0.5}
	}
}

// polyEval evaluates an ascending-coefficient polynomial via Horner.
func polyEval(coeffs []float64, x float64) float64 {
	if len(coeffs) == 0 {
		return 0
	}
	acc := coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		acc = acc*x + coeffs[i]
	}
	return acc
}

// plaintextStep evaluates step(x) = (g^(k)(x)+1)/2 in cleartext, where g is the
// degree-`degree` sign polynomial composed `iters` times. Used by tests to
// validate the design's separation independent of FHE noise.
func plaintextStep(degree, iters int, x float64) float64 {
	cs := signCoeffs(degree)
	v := x
	if iters < 1 {
		iters = 1
	}
	for i := 0; i < iters; i++ {
		v = polyEval(cs, v)
	}
	return (v + 1.0) / 2.0
}

// plaintextMask computes the homomorphic mask's intended cleartext value (the
// product of step polynomials), so tests can compare the circuit's analog
// output to its own design before involving FHE noise.
func plaintextMask(keys []float64, degree, iters int) []float64 {
	n := len(keys)
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		prod := 1.0
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			prod *= plaintextStep(degree, iters, keys[j]-keys[i])
		}
		out[i] = prod
	}
	return out
}
