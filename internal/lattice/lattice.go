// Package lattice is the deterministic feedback-topology simulation that
// tests crystal's riskiest assumption: can a stacked, self-reauthoring tier
// lattice stay convergent when drift signals must propagate UP through
// lossy boundaries?
//
// Model (one chore class, one bottom tier that drifts):
//   - Depth L tiers. The bottom tier has a true error rate `err`.
//   - At InjectStep, `err` jumps to DriftErr (a distribution shift).
//   - The top tier only sees an ATTENUATED signal: observed = err · fidelity,
//     where fidelity = (1−HopLoss)^(L−1). Attenuation under-reports (a lost
//     signal looks like "all fine") — the dangerous direction, and the
//     anti-rigging guard: without loss the loop trivially succeeds.
//   - The top re-authors only what it can see: if observed > DemoteThreshold,
//     it corrects err down by CorrectionGain·observed (you cannot fix a
//     problem you cannot perceive).
//
// Consequence (this is the finding the sim quantifies): correction stalls
// once observed drops to DemoteThreshold, leaving a SILENT FLOOR of
// err ≈ DemoteThreshold / fidelity that the top can no longer see. As depth
// or loss grow, that floor rises — first above the recovery threshold
// (partial silent degradation), then above DriftErr itself (the top never
// even alarms: fully silent). The convergence frontier is therefore
// geometric in depth: fidelity = (1−loss)^(depth−1).
package lattice

import "math"

// Params configures one simulation run.
type Params struct {
	Depth            int     // number of stacked tiers (≥1; 1 = top supervises bottom directly)
	HopLoss          float64 // per-boundary signal loss λ ∈ [0,1)
	DriftErr         float64 // bottom-tier error rate after the injected shift
	CorrectionGain   float64 // how much of the perceived error the re-author removes per step
	DemoteThreshold  float64 // observed error that triggers re-authoring
	RecoverThreshold float64 // bottom error considered "recovered"
	InjectStep       int
	MaxSteps         int
}

// Result is the outcome of a run.
type Result struct {
	Fidelity       float64 // (1−HopLoss)^(Depth−1): signal strength reaching the top
	Detected       bool    // did the top's observed error ever exceed DemoteThreshold?
	Converged      bool    // did bottom error return to ≤ RecoverThreshold?
	StepsToRecover int     // steps after injection to recover; -1 if never
	PeakErr        float64
	FinalErr       float64
	Regime         string // "ok" | "residual" | "silent"
}

// Simulate runs one configuration.
func Simulate(p Params) Result {
	fidelity := math.Pow(1-p.HopLoss, float64(maxi(p.Depth-1, 0)))
	err := 0.0
	peak := 0.0
	detected := false
	stepsToRecover := -1

	for t := 0; t < p.MaxSteps; t++ {
		if t == p.InjectStep {
			err = p.DriftErr
		}
		if err > peak {
			peak = err
		}
		observed := err * fidelity
		if observed > p.DemoteThreshold {
			detected = true
			err -= p.CorrectionGain * observed
			if err < 0 {
				err = 0
			}
		}
		if t >= p.InjectStep && stepsToRecover < 0 && err <= p.RecoverThreshold {
			stepsToRecover = t - p.InjectStep
		}
	}

	res := Result{
		Fidelity:       fidelity,
		Detected:       detected,
		Converged:      err <= p.RecoverThreshold,
		StepsToRecover: stepsToRecover,
		PeakErr:        peak,
		FinalErr:       err,
	}
	switch {
	case !detected:
		res.Regime = "silent" // top never even alarmed — fully silent degradation
	case res.Converged:
		res.Regime = "ok"
	default:
		res.Regime = "residual" // alarmed but signal too weak to fix fully
	}
	return res
}

// Cell is one (depth, loss) point in a sweep.
type Cell struct {
	Depth   int     `json:"depth"`
	HopLoss float64 `json:"hop_loss"`
	Result  Result  `json:"result"`
}

// Sweep runs the simulation across a grid of depths and losses, holding the
// other params fixed. Returns cells in (depth, loss) order.
func Sweep(base Params, depths []int, losses []float64) []Cell {
	var out []Cell
	for _, d := range depths {
		for _, l := range losses {
			p := base
			p.Depth, p.HopLoss = d, l
			out = append(out, Cell{Depth: d, HopLoss: l, Result: Simulate(p)})
		}
	}
	return out
}

// MaxSafeDepth returns the deepest stack that still converges at the given
// loss under base params (0 if none of the tried depths converge).
func MaxSafeDepth(base Params, loss float64, maxDepth int) int {
	safe := 0
	for d := 1; d <= maxDepth; d++ {
		p := base
		p.Depth, p.HopLoss = d, loss
		if Simulate(p).Converged {
			safe = d
		}
	}
	return safe
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}
