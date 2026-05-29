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
	Depth          int     // number of stacked tiers (≥1; 1 = top supervises bottom directly)
	HopLoss        float64 // per-boundary signal loss λ ∈ [0,1) on the FUZZY (NL-summary) channel
	GuardrailCov   float64 // g ∈ [0,1): fraction of drift carried by a LOSSLESS deterministic guardrail
	DriftUncovered bool    // if true, the injected drift sits in the un-guardrailed residual:
	//                         a novel / well-formed-but-wrong failure mode the deterministic
	//                         check was never written for, so the guardrail is BLIND to it (g
	//                         does not help). This is the realistic dangerous case per hard
	//                         rule #2 — the high-value fuzzy drift is exactly the un-checkable part.
	DriftErr         float64 // bottom-tier error rate after the injected shift
	CorrectionGain   float64 // how much of the perceived error the re-author removes per step
	DemoteThreshold  float64 // observed error that triggers re-authoring
	RecoverThreshold float64 // bottom error considered "recovered"
	InjectStep       int
	MaxSteps         int
}

// perceivedFactor is the fraction of the true error the top actually perceives.
// The up-signal is two channels: a deterministic guardrail (coverage g) that is
// LOSSLESS for what it checks, and a fuzzy NL-summary channel that decays as
// (1-λ)^(depth-1). So perceived = err·[ g + (1-g)·fidelity ].
//
// This is the correction to the original over-pessimistic model, which assumed
// g=0 (pure fuzzy channel). At g=1 the supervisor sees true error regardless of
// depth or λ — deterministic guardrails defeat propagation loss. The catch
// (hard rule #2): g is bounded by what is deterministically checkable; the
// residual (1-g) is the irreducible fuzzy loss, and keeping g high as drift
// mutates requires the upper tier to keep AUTHORING new guardrails — a dynamic
// this static model does not capture (it's the live experiment's job to
// measure real g and whether it keeps pace).
func perceivedFactor(g, fidelity float64) float64 {
	return g + (1-g)*fidelity
}

// GuardrailThreshold is the coverage g above which a stack converges at
// UNBOUNDED depth (the silent floor demote/g stays ≤ recover). It is just the
// ratio demote/recover. Below it the safe depth is finite and geometric.
//
// Honest framing (panel-mandated): coverage is a CLIFF at this threshold, not a
// smooth dial — and the "uniform g" model assumes the guardrail catches a
// severity-representative slice of error. For drift in the un-checkable
// residual (DriftUncovered), g does not help at all. Report a band over the
// (unmeasured) demote/recover knobs, never a single depth integer.
func GuardrailThreshold(demote, recover float64) float64 { return demote / recover }

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
		if mag := math.Abs(err); mag > peak {
			peak = mag
		}
		// Perceived signal. If the drift is in the un-guardrailed residual (a
		// novel/well-formed-but-wrong mode the deterministic check was never
		// written for), the guardrail is BLIND to it — only the fuzzy channel
		// carries it, so g does not help. This is the realistic dangerous case
		// (hard rule #2): the high-value drift is exactly the un-checkable part.
		factor := perceivedFactor(p.GuardrailCov, fidelity)
		if p.DriftUncovered {
			factor = (1 - p.GuardrailCov) * fidelity
		}
		observed := err * factor
		// NO clamp at 0: over-correction can overshoot past zero into the
		// opposite-sign error. This makes the canonical control-loop failure
		// (high-gain oscillation/divergence — an over-eager re-author breaking
		// a working harness) expressible, not just under-actuation.
		if math.Abs(observed) > p.DemoteThreshold {
			detected = true
			err -= p.CorrectionGain * observed
		}
		if t >= p.InjectStep && stepsToRecover < 0 && math.Abs(err) <= p.RecoverThreshold {
			stepsToRecover = t - p.InjectStep
		}
	}

	res := Result{
		Fidelity:       fidelity,
		Detected:       detected,
		Converged:      math.Abs(err) <= p.RecoverThreshold,
		StepsToRecover: stepsToRecover,
		PeakErr:        peak,
		FinalErr:       err,
	}
	switch {
	case !detected:
		res.Regime = "silent" // top never even alarmed — fully silent degradation
	case res.Converged:
		res.Regime = "ok"
	case math.Abs(err) > p.DriftErr:
		res.Regime = "unstable" // over-correction amplified error past the injected shift
	default:
		res.Regime = "residual" // alarmed but signal too weak to fix fully
	}
	return res
}

// ClosedFormDepth returns the analytic under-actuation frontier: the largest
// depth d with (1-λ)^(d-1) >= demote/recover. The adversarial panel confirmed
// the sim IS essentially this inequality (closed form predicts 34/36 grid
// cells) plus a small, GAIN-DEPENDENT discrete-overshoot correction near the
// boundary. Report this as algebra, not as an emergent property. Returns -1
// for λ<=0 (converges at all depths).
func ClosedFormDepth(lambda, demote, recover float64) int {
	if lambda <= 0 {
		return -1
	}
	ratio := demote / recover
	if ratio >= 1 {
		return 1
	}
	d := 1 + int(math.Floor(math.Log(ratio)/math.Log(1-lambda)))
	if d < 1 {
		d = 1
	}
	return d
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
