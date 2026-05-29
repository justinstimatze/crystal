package lattice_test

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/lattice"
)

// base holds ILLUSTRATIVE constants. The adversarial panel established that the
// frontier integer is contingent on gain and demote/recover, NOT just λ — so
// these values are not "the answer", they are one corner. The sensitivity
// tests below exist precisely to show the result moves when they move.
func base() lattice.Params {
	return lattice.Params{
		DriftErr:         0.5,
		CorrectionGain:   0.5,
		DemoteThreshold:  0.08,
		RecoverThreshold: 0.10,
		InjectStep:       5,
		MaxSteps:         200,
	}
}

// Sanity: with lossless propagation the self-reauthoring loop converges at
// any depth. (If THIS failed, the model would be broken, not the thesis.)
func TestLosslessAlwaysConverges(t *testing.T) {
	for _, d := range []int{1, 2, 3, 5, 10} {
		p := base()
		p.Depth, p.HopLoss = d, 0.0
		if r := lattice.Simulate(p); !r.Converged {
			t.Errorf("depth=%d lossless: did not converge (finalErr=%.3f)", d, r.FinalErr)
		}
	}
}

// The frontier MUST shallow as loss rises — the one robust qualitative claim.
// (Not "a frontier exists at all" — the panel showed that's a tautology, since
// the silent floor demote/(1-λ)^(d-1) → ∞ while recover is finite, so a failing
// depth always exists for any λ>0.)
func TestFrontierShallowsAsLossRises(t *testing.T) {
	prev := 1 << 30
	for _, l := range []float64{0.1, 0.2, 0.3, 0.4} {
		d := lattice.MaxSafeDepth(base(), l, 12)
		if d > prev {
			t.Errorf("max safe depth should not increase with loss: λ=%.1f depth=%d prev=%d", l, d, prev)
		}
		prev = d
	}
}

// PANEL FINDING #1: the headline integer is GAIN-contingent, not a property of
// the topology. At depth 3 / λ=0.2 the result flips on the (unmeasured) gain.
func TestGainFlipsTheFrontier(t *testing.T) {
	low := lattice.Simulate(lattice.Params{Depth: 3, HopLoss: 0.2, DriftErr: 0.5, CorrectionGain: 0.5, DemoteThreshold: 0.08, RecoverThreshold: 0.10, InjectStep: 5, MaxSteps: 200})
	high := lattice.Simulate(lattice.Params{Depth: 3, HopLoss: 0.2, DriftErr: 0.5, CorrectionGain: 0.9, DemoteThreshold: 0.08, RecoverThreshold: 0.10, InjectStep: 5, MaxSteps: 200})
	if low.Converged {
		t.Errorf("expected gain 0.5 to FAIL at depth 3 / λ0.2 (the pessimistic corner)")
	}
	if !high.Converged {
		t.Errorf("expected gain 0.9 to CONVERGE at depth 3 / λ0.2 — gain flips the headline")
	}
	t.Logf("GAIN-CONTINGENT: depth3/λ0.2 — gain 0.5 final=%.3f (%s), gain 0.9 final=%.3f (%s)",
		low.FinalErr, low.Regime, high.FinalErr, high.Regime)
}

// PANEL FINDING #2: demote/recover is co-equally load-bearing — so "λ is THE
// variable" is false. A small demote nudge moves the integer.
func TestDemoteFlipsTheFrontier(t *testing.T) {
	mk := func(demote float64) lattice.Params {
		p := base()
		p.DemoteThreshold = demote
		return p
	}
	d05 := lattice.MaxSafeDepth(mk(0.05), 0.2, 12)
	d08 := lattice.MaxSafeDepth(mk(0.08), 0.2, 12)
	d09 := lattice.MaxSafeDepth(mk(0.09), 0.2, 12)
	if !(d05 > d08 && d08 >= d09) {
		t.Errorf("demote should move the frontier: d(0.05)=%d d(0.08)=%d d(0.09)=%d", d05, d08, d09)
	}
	t.Logf("DEMOTE-CONTINGENT: λ0.2 max safe depth — demote 0.05→%d, 0.08→%d, 0.09→%d", d05, d08, d09)
}

// PANEL FINDING #3 (now expressible): high gain over-corrects and destabilizes
// — the over-eager-re-author-breaks-a-working-harness failure, impossible to
// see under the old err>=0 clamp.
func TestOverCorrectionIsUnstable(t *testing.T) {
	p := base()
	p.Depth, p.HopLoss, p.CorrectionGain = 1, 0.0, 5.0 // fidelity 1, wildly over-eager
	r := lattice.Simulate(p)
	if r.Converged {
		t.Error("gain 5 at fidelity 1 should NOT converge — it over-corrects")
	}
	if r.Regime != "unstable" {
		t.Errorf("regime=%s, want unstable (over-correction amplifies error)", r.Regime)
	}
	t.Logf("OVER-CORRECTION: gain 5 → regime=%s finalErr=%.2f (oscillating divergence, now expressible)", r.Regime, r.FinalErr)
}

// The frontier IS the algebra: at the published grid's gain (0.5), the
// closed-form ratio demote/recover predicts the simulated frontier within the
// ±1 discrete-overshoot the panel found (34/36 cells). This is the point — the
// "frontier" is the inequality (1-λ)^(d-1) >= demote/recover restated, not an
// emergent topological property. (Higher gain overshoots the floor and
// converges deeper, which is exactly the gain-contingency TestGainFlips shows.)
func TestClosedFormTracksFrontier(t *testing.T) {
	p := base() // gain 0.5 — the grid's gain
	for _, l := range []float64{0.1, 0.2, 0.3} {
		sim := lattice.MaxSafeDepth(p, l, 20)
		cf := lattice.ClosedFormDepth(l, p.DemoteThreshold, p.RecoverThreshold)
		if diff := sim - cf; diff < -1 || diff > 1 {
			t.Errorf("λ=%.1f: sim depth %d vs closed form %d (off by >1 — frontier isn't the ratio?)", l, sim, cf)
		}
		t.Logf("λ=%.1f: sim=%d closed-form=%d (frontier = algebra, ±1 discretization)", l, sim, cf)
	}
}

// Deeper or lossier strictly weakens the signal reaching the top (monotone),
// so the frontier is well-ordered, not noisy.
func TestFidelityMonotoneInDepthAndLoss(t *testing.T) {
	prev := 1.01
	for _, d := range []int{1, 2, 3, 4, 5} {
		f := lattice.Simulate(withDepthLoss(base(), d, 0.3)).Fidelity
		if f >= prev {
			t.Errorf("fidelity not decreasing with depth: depth=%d f=%.3f prev=%.3f", d, f, prev)
		}
		prev = f
	}
}

// The fully-silent regime exists: enough depth×loss and the top never even
// alarms — the undercity nightmare, reproduced structurally.
func TestSilentRegimeExists(t *testing.T) {
	r := lattice.Simulate(withDepthLoss(base(), 6, 0.5)) // fidelity = 0.5^5 ≈ 0.031
	if r.Detected {
		t.Errorf("expected fully-silent (top never alarms) at depth 6 / loss 0.5, but Detected=true (fidelity=%.3f)", r.Fidelity)
	}
	if r.Regime != "silent" {
		t.Errorf("regime=%s, want silent", r.Regime)
	}
	t.Logf("SILENT REGIME: depth 6 / loss 0.5 → fidelity %.3f, top never alarms, bottom stuck at %.2f", r.Fidelity, r.FinalErr)
}

func withDepthLoss(p lattice.Params, d int, l float64) lattice.Params {
	p.Depth, p.HopLoss = d, l
	return p
}

// IT IS NOT ALL LOSS (the user's point): a deterministic guardrail emits a
// LOSSLESS structured signal for what it covers. At full coverage (g=1) the
// supervisor perceives true error regardless of depth or λ — so a stack that
// silently fails on the fuzzy NL channel converges fine on the guardrail
// channel. This is the correction to the original over-pessimistic g=0 model.
func TestGuardrailCoverageDefeatsLoss(t *testing.T) {
	// A configuration that FAILS on the pure fuzzy channel (g=0):
	fail := withDepthLoss(base(), 6, 0.5) // fidelity ≈ 0.03 → silent
	if r := lattice.Simulate(fail); r.Converged {
		t.Fatal("precondition: depth 6 / λ0.5 should fail at g=0")
	}
	// Same depth/λ, but a deterministic guardrail carries the signal losslessly:
	full := fail
	full.GuardrailCov = 1.0
	if r := lattice.Simulate(full); !r.Converged {
		t.Errorf("g=1 (lossless deterministic guardrail) should converge even at depth 6 / λ0.5, got %s finalErr=%.3f", r.Regime, r.FinalErr)
	}
}

// PANEL CORRECTION: guardrail coverage is a CLIFF at g = demote/recover, NOT a
// smooth dial — and a MaxSafeDepth result equal to the search cap means
// "unbounded", not a frontier. Below the threshold the depth is finite and
// cap-INVARIANT; above it the result saturates the cap at every cap. The old
// "g=0.6→6, g=0.9→30" series was a search-cap artifact (30 = the maxDepth arg).
func TestGuardrailCoverageIsAThresholdNotADial(t *testing.T) {
	mk := func(g float64) lattice.Params { p := base(); p.GuardrailCov = g; return p }
	thr := lattice.GuardrailThreshold(base().DemoteThreshold, base().RecoverThreshold) // 0.8
	// Below threshold: finite and cap-invariant (same answer at cap 50 and 500).
	below := mk(thr - 0.2)
	d50, d500 := lattice.MaxSafeDepth(below, 0.3, 50), lattice.MaxSafeDepth(below, 0.3, 500)
	if d50 != d500 || d50 >= 50 {
		t.Errorf("below threshold should be finite + cap-invariant, got cap50=%d cap500=%d", d50, d500)
	}
	// Above threshold: unbounded — saturates whatever cap you pass.
	above := mk(thr + 0.15)
	if lattice.MaxSafeDepth(above, 0.3, 100) != 100 || lattice.MaxSafeDepth(above, 0.3, 1000) != 1000 {
		t.Errorf("above threshold should saturate the cap (unbounded), not return a frontier")
	}
	t.Logf("threshold g=%.2f: below→finite depth %d (cap-invariant); above→unbounded (cap-saturated)", thr, d50)
}

// PANEL DEFECT #2: if the dangerous drift sits in the un-checkable residual,
// the guardrail is blind to it — high g does NOT save you. This is the
// realistic case (hard rule #2: the valuable fuzzy drift is the un-checkable
// part). The optimistic "uniform g" result is an upper bound, not the truth.
func TestUncoveredDriftDefeatsHighCoverage(t *testing.T) {
	p := withDepthLoss(base(), 6, 0.5) // fidelity ≈ 0.03
	p.GuardrailCov = 0.9               // would "converge" under the uniform-blend model
	p.DriftUncovered = true            // ...but this drift mode is in the un-guardrailed tail
	if r := lattice.Simulate(p); r.Converged {
		t.Errorf("uncovered drift at depth6/λ0.5 must NOT converge despite g=0.9 (guardrail is blind to it), got %s", r.Regime)
	}
}
