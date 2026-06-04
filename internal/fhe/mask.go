// SPDX-License-Identifier: Apache-2.0

//go:build openfhe

package fhe

import (
	"fmt"

	"github.com/Fheyalabs/ares-core/pkg/ares/crypto/fhecalib"
)

// Encrypted-input layout produced by ArgminCircuit.Inputs(). Eval receives the
// ciphertexts in this order.
const (
	inKeys     = 0 // packed key vector [k_0..k_{n-1}] (used only for shape)
	inZeros    = 1 // ENC(0) broadcast: builds differences, negations, scalars
	inLeadSeed = 2 // ENC(g's leading coeff) broadcast: Horner seed (a scalar ct)
)

// Eval runs the homomorphic argmin one-hot using ONLY the fhecalib
// ContextHandle primitives: EvalMult (ct*ct, 1 level) and EvalSubConst
// (ct - publicVec, 0 levels). The handle exposes no scalar-multiply and no
// ciphertext+ciphertext addition, so:
//
//   - Scalar coefficients are applied by multiplying against an ENCRYPTED
//     constant (supplied via Inputs()), each costing one multiplicative level.
//   - Polynomials are evaluated in Horner form (only ct*ct mults and
//     add-CONSTANT folds via EvalSubConst with a negated constant), never a
//     ciphertext+ciphertext sum. Horner depth == polynomial degree, which is
//     why the sharp step is built by COMPOSING a small degree-3 g, not by one
//     high-degree polynomial.
//   - The cross-candidate product is a balanced EvalMult tree, never a
//     cross-slot reduction (EvalProductSum only sums, which would destroy the
//     one-hot slot structure).
//
// Slot i of the returned ciphertext equals Π_{j!=i} step(k_j - k_i): ~1 for the
// minimum key, ~0 otherwise.
//
// Depth ledger (g degree 3, k = SignIters, n keys):
//
//	differences      : 0 levels (EvalSubConst only)
//	one g application : 3 levels (Horner over degree 3: seed*x, *x, *x)
//	k compositions    : 3k levels
//	step map (*0.5)   : +1 level
//	product tree      : +ceil(log2 n) levels
//
// so total measured depth ~ 3k + 1 + ceil(log2 n).
func (c ArgminCircuit) Eval(h fhecalib.ContextHandle, encInputs [][]byte) ([]byte, error) {
	n := len(c.Keys)
	if n < 2 {
		return nil, fmt.Errorf("argmin: need >=2 keys, got %d", n)
	}
	if len(encInputs) < 3 {
		return nil, fmt.Errorf("argmin: expected 3 encrypted inputs, got %d", len(encInputs))
	}
	encZeros := encInputs[inZeros]
	encSeed := encInputs[inLeadSeed]

	// negK slot i = -k_i, built with zero multiplicative levels.
	negK, err := h.EvalSubConst(encZeros, c.Keys)
	if err != nil {
		return nil, fmt.Errorf("argmin: negate keys: %w", err)
	}

	// stepCT[j] slot i = step(k_j - k_i): ~1 when k_j > k_i (i wins the pair).
	// The self-comparison slot j (diff 0) gives step(0)=0.5, an unwanted factor
	// in the global product; we pin slot j to exactly 1 with a single
	// add-constant (zero levels) so it is a product no-op -- no per-slot
	// pruning, no final scalar correction, no extra multiplicative level.
	stepCT := make([][]byte, n)
	for j := 0; j < n; j++ {
		// diffJ slot i = -k_i + k_j = k_j - k_i (zero levels: add-const only).
		diffJ, err := h.EvalSubConst(negK, fill(n, -c.Keys[j]))
		if err != nil {
			return nil, fmt.Errorf("argmin: difference row %d: %w", j, err)
		}
		s, err := evalStep(h, diffJ, encZeros, encSeed, c.SignDegree, c.iters(), n)
		if err != nil {
			return nil, fmt.Errorf("argmin: step row %d: %w", j, err)
		}
		// Pin self-factor at slot j: slot j currently step(0)=0.5 -> -(0.5-1)=+0.5 -> 1.
		selfFix := make([]float64, n)
		selfFix[j] = stepZeroValue() - 1.0
		s, err = h.EvalSubConst(s, selfFix)
		if err != nil {
			return nil, fmt.Errorf("argmin: pin self slot row %d: %w", j, err)
		}
		stepCT[j] = s
	}

	// mask_i = Π_j stepCT[j] at slot i (self-factors pinned to 1).
	out, err := balancedProduct(h, stepCT)
	if err != nil {
		return nil, fmt.Errorf("argmin: product tree: %w", err)
	}
	return out, nil
}

// stepZeroValue is step(0) = (g^k(0)+1)/2. g is odd so g^k(0)=0, hence 0.5 for
// every degree and iteration count.
func stepZeroValue() float64 { return 0.5 }

// evalStep computes step(x) = (g^k(x)+1)/2 on the per-slot ciphertext x using
// ONLY EvalMult and EvalSubConst:
//   - g is composed k times via repeated Horner evaluation of signCoeffs.
//   - The leading coefficient of the FIRST g is the encrypted seed (a real
//     ct*ct EvalMult). Subsequent iterations re-introduce the leading scalar by
//     deriving ENC(c_lead) from ENC(0) with a zero-level add-constant.
//   - The final [-1,1]->[0,1] map multiplies by ENC(0.5) (one level) and folds
//     +0.5 as an add-constant.
func evalStep(h fhecalib.ContextHandle, x, encZeros, encSeed []byte, degree, iters, slots int) ([]byte, error) {
	coeffs := signCoeffs(degree)
	lead := coeffs[len(coeffs)-1]

	v := x
	for it := 0; it < iters; it++ {
		// seed = ENC(lead): reuse the supplied seed on the first iteration,
		// rebuild from ENC(0) afterward (ENC(0) - (-lead) = ENC(lead), 0 levels).
		seed := encSeed
		if it > 0 {
			s, err := h.EvalSubConst(encZeros, fill(slots, -lead))
			if err != nil {
				return nil, fmt.Errorf("step: rebuild seed (it=%d): %w", it, err)
			}
			seed = s
		}
		g, err := hornerEval(h, v, seed, coeffs, slots)
		if err != nil {
			return nil, fmt.Errorf("step: g (it=%d): %w", it, err)
		}
		v = g
	}

	// map [-1,1] -> [0,1]: 0.5*v + 0.5.
	encHalf, err := h.EvalSubConst(encZeros, fill(slots, -0.5)) // ENC(0.5)
	if err != nil {
		return nil, fmt.Errorf("step: build ENC(0.5): %w", err)
	}
	scaled, err := h.EvalMult(v, encHalf)
	if err != nil {
		return nil, fmt.Errorf("step: scale 0.5: %w", err)
	}
	mapped, err := h.EvalSubConst(scaled, fill(slots, -0.5)) // +0.5
	if err != nil {
		return nil, fmt.Errorf("step: add 0.5: %w", err)
	}
	return mapped, nil
}

// hornerEval evaluates sum coeffs[k] x^k (ascending) on ciphertext x via
// Horner, using ONLY EvalMult (ct*ct) and EvalSubConst (add-const). seed is
// ENC(coeffs[last]).
//
//	acc = c_last (seed); for k=last-1..0: acc = acc*x; acc = acc + c_k
func hornerEval(h fhecalib.ContextHandle, x, seed []byte, coeffs []float64, slots int) ([]byte, error) {
	acc := seed
	for k := len(coeffs) - 2; k >= 0; k-- {
		m, err := h.EvalMult(acc, x)
		if err != nil {
			return nil, fmt.Errorf("horner mult (k=%d): %w", k, err)
		}
		acc = m
		if coeffs[k] != 0 {
			a, err := h.EvalSubConst(acc, fill(slots, -coeffs[k]))
			if err != nil {
				return nil, fmt.Errorf("horner add-const (k=%d): %w", k, err)
			}
			acc = a
		}
	}
	return acc, nil
}

// balancedProduct returns the slotwise product of all ciphertexts via a
// balanced binary tree, costing ceil(log2(len)) multiplicative levels.
func balancedProduct(h fhecalib.ContextHandle, cts [][]byte) ([]byte, error) {
	if len(cts) == 0 {
		return nil, fmt.Errorf("balancedProduct: no factors")
	}
	level := make([][]byte, len(cts))
	copy(level, cts)
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				next = append(next, level[i])
				continue
			}
			m, err := h.EvalMult(level[i], level[i+1])
			if err != nil {
				return nil, fmt.Errorf("balancedProduct mult: %w", err)
			}
			next = append(next, m)
		}
		level = next
	}
	return level[0], nil
}
