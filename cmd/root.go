// Package cmd is the crystal CLI. Phase 1 exposes two subcommands:
// `extract` (build redacted Record fixtures from local transcripts) and
// `eval` (replay a synthetic artifact over a corpus and print the report).
package cmd

import (
	"errors"

	"github.com/alecthomas/kong"
)

// CLI is the root command struct.
type CLI struct {
	Extract      ExtractCmd      `cmd:"" help:"Walk Claude Code transcripts into a redacted, per-tool-balanced Record corpus."`
	Eval         EvalCmd         `cmd:"" help:"Replay a synthetic artifact over a corpus and print per-tool fidelity reports."`
	Measure      MeasureCmd      `cmd:"" help:"Sweep signature granularities over the full substrate to find crystallizable (frequent AND deterministic) patterns."`
	Drift        DriftCmd        `cmd:"" help:"Temporal-replay drift experiment: promote a modal hook on a pattern's early occurrences, stream the rest, report demotion and silent-wrong leakage."`
	Crystallize  CrystallizeCmd  `cmd:"" help:"Full lifecycle on one pattern: discover → propose → promote-gate → serve → drift-monitor → demote; emits a redacted deployable artifact."`
	Lattice      LatticeCmd      `cmd:"" help:"Deterministic feedback-topology sim: depth × per-hop-loss convergence grid for the self-reauthoring tier stack (the riskiest-assumption test, no API cost)."`
	Probe        ProbeCmd        `cmd:"" help:"One cheap live API call to confirm the tier plumbing (key from .env, SDK, disk cache) works."`
	Experiment   ExperimentCmd   `cmd:"" help:"Live grounding: per-tier substitution fidelity, fuzzy-channel λ, and deterministic guardrail coverage g on a verifiable chore."`
	GroundHop    GroundHopCmd    `cmd:"" help:"Minimal grounding hop: contrast a deterministic typed up-channel vs a prose up-channel on real records with ground-truth-by-construction drift labels; measures λ and g validly."`
	UncoverHop   UncoverHopCmd   `cmd:"" help:"Uncovered-drift hop: inject semantic errors a deterministic check can't catch (g<1), then measure how much of the residual a fuzzy channel recovers and how lossy one prose hop is."`
	DepthSweep   DepthSweepCmd   `cmd:"" help:"Relay the prose up-channel through k lossy paraphrase hops; measure whether catching-power on uncovered drift compounds-loses over depth (tests the lattice's shallow-safe-depth claim)."`
	ContentSweep ContentSweepCmd `cmd:"" help:"Loop-closer for depth-sweep: recover the proposed correction from each depth-d report and score its fidelity (gold/inverted/other) vs hard labels — the measured content-erosion curve."`
	Payoff       PayoffCmd       `cmd:"" help:"Measure the value prop: shift a mechanical chore Opus→Haiku behind a deterministic gate; report latency saved vs quality held (and leaked) vs always-Opus."`
	Decompose    DecomposeCmd    `cmd:"" help:"A4: does a cheap model + a robust tool (rg) beat shifting the whole chore to the cheap model? Quote-verification, whole-haiku vs det-tool vs haiku+tool."`
	Support      SupportCmd      `cmd:"" help:"The residual experiment: semantic support (does the source back the claim, often via paraphrase) — where a string tool can't win. opus/haiku/det/haiku+retrieval."`
	Aggregate    AggregateCmd    `cmd:"" help:"Hunt the cheap-model limit: semantic aggregation (count how many of N items match a criterion). whole-task vs map-reduce (cheap per-item classify + deterministic count)."`
	Triage       TriageCmd       `cmd:"" help:"v1 SLICE: map-reduce + verifier on a real chore — categorize your actual Bash usage. Deterministic rules cover+gate; cheap model does the residual; deterministic tally. No frontier."`
	Author       AuthorCmd       `cmd:"" help:"Self-author the verifier: the expensive tier writes triage's deterministic rule table from labeled examples, gated on a holdout (corrupted rules rejected), re-authored when a new command class drifts in."`
}

// Exit codes: 0 ok, 2 input/usage error, 1 fatal.
const (
	ExitOK    = 0
	ExitInput = 2
	ExitFatal = 1
)

// usageError marks an error as a bad-input (exit 2) condition.
type usageError struct{ err error }

func (u usageError) Error() string { return u.err.Error() }

// ExitCode maps an error returned by a subcommand Run to a process code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var ue usageError
	if errors.As(err, &ue) {
		return ExitInput
	}
	return ExitFatal
}

var _ = kong.Parse // ensure kong import is used by main
