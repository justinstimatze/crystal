// Package crystallize runs the full crystallization lifecycle on a single
// pattern end-to-end — the project's proof-of-loop:
//
//	discover → propose → promote-gate → serve → drift-monitor → demote
//
// It ties together the components built separately: artifact.Modal (the
// proposer's output for a constant/modal-output pattern), the eval promote
// gate (determinism ≥ threshold, N ≥ minimum), and the drift detector's
// windowed demotion rule. It proves the loop's LOGIC closes on real data
// and emits a deployable Spec; it does not itself wire live Claude Code
// interception (Phase 2/4).
//
// Note on the proposer: for the constant-output patterns that are actually
// crystallizable (see docs/MEASURE_FINDINGS.md), the proposer is
// deterministic — "serve the cached output, verify with the tool's
// comparator." No LLM is needed. The brief's single-Opus-call proposer is
// only required for fuzzier targets, which the substrate does not surface.
package crystallize

import (
	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/drift"
	"github.com/justinstimatze/crystal/internal/eval"
	"github.com/justinstimatze/crystal/internal/record"
)

// DemotionRule is the live drift rule baked into a promoted artifact:
// demote on M divergences within a sliding window of W (see
// docs/DRIFT_FINDINGS.md for why consecutive-K is insufficient).
type DemotionRule struct {
	M int `json:"m"`
	W int `json:"w"`
}

// Spec is the deployable crystallized artifact plus its lifecycle outcome.
type Spec struct {
	Pattern          string        `json:"pattern"`
	Tool             string        `json:"tool"`
	Verifier         string        `json:"verifier"` // comparator used for drift checks
	Promoted         bool          `json:"promoted"`
	PromoteDecision  string        `json:"promote_decision"` // promote | reject | insufficient | unverifiable
	TrainN           int           `json:"train_n"`
	TrainDeterminism float64       `json:"train_determinism"`
	ServedOutput     record.Output `json:"served_output"`
	DemotionRule     DemotionRule  `json:"demotion_rule"`

	// Lifecycle outcome from monitored serving over the holdout stream.
	HoldoutN       int    `json:"holdout_n"`
	ServedCorrect  int    `json:"served_correct"`
	Leaked         int    `json:"leaked"`
	DemotedAtIndex int    `json:"demoted_at_index"`
	ServeDecision  string `json:"serve_decision"` // served-clean | demoted | leaked-without-demote | n/a
}

// Run executes the lifecycle on one pattern's time-ordered records.
// recs MUST be a single exact-signature group (the caller picks it), sorted
// by timestamp. m/w are the live demotion rule.
func Run(pattern string, recs []record.Record, trainFrac float64, m, w int) Spec {
	tool := ""
	if len(recs) > 0 {
		tool = recs[0].Tool
	}
	spec := Spec{Pattern: pattern, Tool: tool, Verifier: tool, DemotionRule: DemotionRule{M: m, W: w}, DemotedAtIndex: -1, ServeDecision: "n/a"}

	cut := int(float64(len(recs)) * trainFrac)
	if cut < 1 {
		cut = 1
	}
	train, holdout := recs[:cut], recs[cut:]

	// PROPOSE: build the modal hook from training records.
	art, det := artifact.NewModal(pattern, train)
	spec.TrainN = len(train)
	spec.TrainDeterminism = det
	spec.ServedOutput, _ = art.Produce(record.Record{}) // the cached output it would serve

	// PROMOTE GATE: determinism ≥ threshold AND enough samples.
	switch {
	case len(train) < eval.MinSamples:
		spec.PromoteDecision = "insufficient"
	case det >= eval.PromoteThreshold:
		spec.PromoteDecision = "promote"
		spec.Promoted = true
	default:
		spec.PromoteDecision = "reject"
	}
	if !spec.Promoted {
		return spec // refused — nothing is deployed; fail loud at the caller
	}

	// SERVE + MONITOR: replay the holdout under the windowed demotion rule.
	rep := drift.Replay(pattern, tool, train, holdout, m, w, eval.PromoteThreshold, false)
	spec.HoldoutN = rep.StreamN
	spec.ServedCorrect = rep.ServedCorrect
	spec.Leaked = rep.Leaked
	spec.DemotedAtIndex = rep.DemotedAtIndex
	spec.ServeDecision = rep.Decision
	return spec
}
