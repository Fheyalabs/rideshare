# Price-only argmin: real-FHE depth calibration

Calibration of the packed one-hot **price argmin** circuit (`ArgminCircuit`) as
an `ares-core/pkg/ares/crypto/fhecalib.CircuitUnderTest`, measured against the
real OpenFHE 1.5.1 backend (`-tags openfhe`).

The circuit emits, for a cohort of `n` normalized keys in `[0,1]` packed into
slots `0..n-1`, the one-hot mask of the minimum key:

    mask_i = Π_{j != i} step(k_j - k_i)

where `step(x) ~ 1` when `k_j > k_i` (so `i` is the smaller of the pair) and
`~ 0` otherwise. The product is `~1` only for the global minimum.

## ContextHandle primitive set (the binding constraint)

`fhecalib.ContextHandle` exposes exactly three homomorphic ops:

| method | semantics | mult. levels |
|---|---|---|
| `EvalMult(a, b)` | slotwise `a * b` | +1 |
| `EvalSubConst(ct, vals)` | `ct - publicVec` | 0 |
| `EvalProductSum(l, r, n)` | `EvalSum(EvalMult(l, r), n)` (cross-slot **sum**) | +1 |

There is **no scalar-multiply** and **no ciphertext+ciphertext addition** on the
handle. This dictates the entire construction:

- **Scalars** (polynomial coefficients) are applied by multiplying against an
  *encrypted constant* supplied through `Inputs()` — each costs one level.
- **`EvalSubConst`** doubles as add-constant (subtract a negated constant), and
  builds all pairwise differences with **zero** levels: with `Z = ENC(0)` and
  plaintext-known keys, `Diff_j[i] = (Z - keys)[i] - (-k_j) = k_j - k_i`.
- **No ct+ct add** ⇒ polynomials can only be evaluated in **Horner form**, whose
  multiplicative depth equals the polynomial **degree**. Paterson–Stockmeyer /
  power-basis evaluation (depth `~log(degree)`) is impossible because it needs
  to *sum* monomials. So a sharp sign function is built by **composing** a small
  degree-3 polynomial, not by one high-degree polynomial.
- `EvalProductSum` sums across slots, which would destroy the one-hot slot
  structure, so the cross-candidate product is a **balanced `EvalMult` tree**,
  never a slot reduction.

### sign approximation and sharpening

`step(x) = (g^(k)(x) + 1) / 2`, where `g(x) = 1.5x - 0.5x³` (the Newton sign
step) is composed `k = SignIters` times. Composition sharpens the transition
band toward a true step. A single application (`k=1`) suffices for `n=2`, but
larger cohorts have a *second-smallest* candidate that loses only one pairwise
comparison; a soft `step < 1` leaves that slot non-negligible, so more
iterations are required to drive it to `~0`.

The scalar-free 2-level alternative `h(x) = 3x - x³` was tested and is
**unstable** on `[-1,1]` (`|h(x)|` can exceed 1, iteration diverges), so the
3-levels-per-iteration `1.5x - 0.5x³` is the real floor.

### depth ledger

    differences           : 0 levels
    one g application      : 3 levels   (Horner over degree 3)
    k compositions         : 3k levels
    step map (× 0.5)       : +1 level
    product tree (n terms) : +ceil(log2 n) levels
    ----------------------------------------------
    total                  : 3k + 1 + ceil(log2 n)

## Results

Tolerance: **0.15** max abs error per output slot vs the exact one-hot.
ScalingModSize: 50. Sign polynomial degree: 3.

The minimum sign-composition count `k` per cohort size (the point at which the
*analog* mask is within tolerance of the one-hot, verified noise-free by
`TestPlaintextMaskWithinTolerance`) and the resulting minimum CKKS
multiplicative depth:

| n | min iters k | min depth (`3k+1+⌈log2 n⌉`) | required ring | worst slot err | real-FHE measured here |
|---|---|---|---|---|---|
| 2 | 1 | **5**  | 2^15 | 0.104 | **YES — depth 5, ring 32768, bestErr 0.1040** |
| 3 | 3 | **12** | 2^16 | 0.100 | RAM-gated (predicted) |
| 4 | 4 | **15** | 2^16 | 0.105 | RAM-gated (predicted) |
| 5 | 4 | **16** | 2^16 | 0.112 | RAM-gated (predicted) |

`n=2` is measured end-to-end on real FHE in every `-tags openfhe` run:

    n=2 iters=1 ring=32768 -> depth=5 passed=true bestAbsErr=0.1040

### Why n>=3 is RAM-gated, not depth-faked

Measured OpenFHE context limits on this machine (HEStd_NotSet, firstMod 60,
scalingMod 50):

- **ring 2^15 admits multiplicative depth ≤ 11** — context creation *fails*
  above that. So `n=2` (depth 5) fits comfortably, but `n>=3` (depth ≥12) does
  **not** fit in ring 2^15.
- **ring 2^16** admits the required depth, but a two-party **threshold keygen**
  at ring 2^16 / depth 12 consumes **~6–9 GB RAM** and was OOM-killed here. RAM
  (not latency) is the binding constraint at this regime, consistent with the
  workspace keygen-amortization findings.

Clean per-config measurements (each in a fresh process to avoid OpenFHE
process-global CKKS state corruption) confirmed the approximation-quality wall
independently of the RAM wall: at `k=2`, ring 2^15, the best achievable abs
error is **0.218 / 0.389 / 0.441** for `n=3 / 4 / 5` — all above 0.15. So `k=2`
genuinely cannot converge regardless of depth; `k>=3` is required and forces
ring 2^16.

For `n>=3` the calibration test asserts the recorded config and **skips the
real-FHE run unless `ARGMIN_BIG_RING=1`** (run on a ≥16 GB host):

    ARGMIN_BIG_RING=1 go test -tags openfhe ./internal/fhe/ -run TestCalibrate -v

## Verdict on the "depth ≤ 8" hypothesis

The design spec's `depth ≤ 8` bound is **not met for n ≥ 3** under this
ContextHandle primitive set. The exact argmin depth was an OPEN spec item; the
measured/derived answer is:

- `n=2`: depth **5** (measured, real FHE).
- `n=3`: depth **12**; `n=4`: depth **15**; `n=5`: depth **16** (each verified
  correct in plaintext at the stated iteration count; real-FHE confirmation is
  RAM-gated to ring 2^16).

The dominant cost is the absence of free scalar-multiply and ciphertext+
ciphertext addition on the handle, which forces Horner evaluation (depth =
degree) and 3 levels per sign-sharpening iteration. A handle that exposed
`EvalAdd` (ct+ct) and a level-free `EvalConstMult` would allow Paterson–
Stockmeyer evaluation and cut the per-iteration cost roughly in half, very
likely bringing `n ≤ 5` under depth 8.

## Files

- `argmin_circuit.go` — `ArgminCircuit` (`CircuitUnderTest`): inputs layout,
  exact one-hot `Expected`, and the shared plaintext sign/step/mask reference.
- `mask.go` (`openfhe`) — `Eval`: the homomorphic mask built from only the
  three ContextHandle primitives (composed-sign Horner + balanced product +
  zero-level self-factor neutralization).
- `argmin_circuit_test.go` — plaintext correctness oracle (runs without
  OpenFHE): one-hot exactness, mask-within-tolerance at `minItersByN`.
- `argmin_calib_openfhe_test.go` (`openfhe`) — the real-FHE depth calibration.
