package lattice_test

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/lattice"
)

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

// THE riskiest-assumption result: with realistic per-hop loss, the loop
// stops converging beyond some depth — silent degradation emerges from the
// topology alone, before any model is involved.
func TestLossyPropagationHasAConvergenceFrontier(t *testing.T) {
	p := base()
	p.HopLoss = 0.2

	// depth 1 (top supervises bottom directly, fidelity=1) must converge.
	p.Depth = 1
	if !lattice.Simulate(p).Converged {
		t.Fatal("depth 1 must converge at any loss (fidelity=1)")
	}

	// There must exist a depth at which it fails — otherwise the sim is rigged.
	maxSafe := lattice.MaxSafeDepth(base(), 0.2, 12)
	if maxSafe >= 12 {
		t.Fatalf("no convergence frontier found up to depth 12 — sim is not testing the risk")
	}
	deepFail := lattice.Simulate(withDepthLoss(base(), maxSafe+1, 0.2))
	if deepFail.Converged {
		t.Errorf("depth %d should NOT converge at loss 0.2", maxSafe+1)
	}
	t.Logf("FRONTIER: at per-hop loss 0.2, max safe stack depth = %d (depth %d fails: regime=%s, finalErr=%.3f)",
		maxSafe, maxSafe+1, deepFail.Regime, deepFail.FinalErr)
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
