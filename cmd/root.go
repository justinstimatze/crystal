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
	Extract       ExtractCmd       `cmd:"" help:"Walk Claude Code transcripts into a redacted, per-tool-balanced Record corpus."`
	SynthCorpus   SynthCorpusCmd   `cmd:"" name:"synth-corpus" help:"Generate the committed test corpus deterministically with invented, schema-faithful content (no real transcripts) — what ships in the public repo so the eval gate runs in CI without leaking. Real-record replay is local-only via extract."`
	Eval          EvalCmd          `cmd:"" help:"Replay a synthetic artifact over a corpus and print per-tool fidelity reports."`
	Bench         BenchCmd         `cmd:"" help:"The single citable quantitative report: verifier-gate integrity (sensitivity/specificity/determinism) over the committed corpus — key-free, reproducible by anyone — plus the executor × placement × openness menu with each tier's reproducibility class. The number a skeptic asks for, in one command."`
	Measure       MeasureCmd       `cmd:"" help:"Sweep signature granularities over the full substrate to find crystallizable (frequent AND deterministic) patterns."`
	Drift         DriftCmd         `cmd:"" help:"Temporal-replay drift experiment: promote a modal hook on a pattern's early occurrences, stream the rest, report demotion and silent-wrong leakage."`
	Crystallize   CrystallizeCmd   `cmd:"" help:"Full lifecycle on one pattern: discover → propose → promote-gate → serve → drift-monitor → demote; emits a redacted deployable artifact."`
	Lattice       LatticeCmd       `cmd:"" help:"Deterministic feedback-topology sim: depth × per-hop-loss convergence grid for the self-reauthoring tier stack (the riskiest-assumption test, no API cost)."`
	Probe         ProbeCmd         `cmd:"" help:"One cheap live API call to confirm the tier plumbing (key from .env, SDK, disk cache) works."`
	Experiment    ExperimentCmd    `cmd:"" help:"Live grounding: per-tier substitution fidelity, fuzzy-channel λ, and deterministic guardrail coverage g on a verifiable chore."`
	GroundHop     GroundHopCmd     `cmd:"" help:"Minimal grounding hop: contrast a deterministic typed up-channel vs a prose up-channel on real records with ground-truth-by-construction drift labels; measures λ and g validly."`
	UncoverHop    UncoverHopCmd    `cmd:"" help:"Uncovered-drift hop: inject semantic errors a deterministic check can't catch (g<1), then measure how much of the residual a fuzzy channel recovers and how lossy one prose hop is."`
	DepthSweep    DepthSweepCmd    `cmd:"" help:"Relay the prose up-channel through k lossy paraphrase hops; measure whether catching-power on uncovered drift compounds-loses over depth (tests the lattice's shallow-safe-depth claim)."`
	ContentSweep  ContentSweepCmd  `cmd:"" help:"Loop-closer for depth-sweep: recover the proposed correction from each depth-d report and score its fidelity (gold/inverted/other) vs hard labels — the measured content-erosion curve."`
	Payoff        PayoffCmd        `cmd:"" help:"Measure the value prop: shift a mechanical chore Opus→Haiku behind a deterministic gate; report latency saved vs quality held (and leaked) vs always-Opus."`
	Decompose     DecomposeCmd     `cmd:"" help:"A4: does a cheap model + a robust tool (rg) beat shifting the whole chore to the cheap model? Quote-verification, whole-haiku vs det-tool vs haiku+tool."`
	Support       SupportCmd       `cmd:"" help:"The residual experiment: semantic support (does the source back the claim, often via paraphrase) — where a string tool can't win. opus/haiku/det/haiku+retrieval."`
	Aggregate     AggregateCmd     `cmd:"" help:"Hunt the cheap-model limit: semantic aggregation (count how many of N items match a criterion). whole-task vs map-reduce (cheap per-item classify + deterministic count)."`
	Triage        TriageCmd        `cmd:"" help:"v1 SLICE: map-reduce + verifier on a real chore — categorize your actual Bash usage. Deterministic rules cover+gate; cheap model does the residual; deterministic tally. No frontier."`
	Author        AuthorCmd        `cmd:"" help:"Self-author the verifier: the expensive tier writes triage's deterministic rule table from labeled examples, gated on a holdout (corrupted rules rejected), re-authored when a new command class drifts in."`
	Serve         ServeCmd         `cmd:"" help:"Measure the payoff: serve the deterministic tier in place of the model on the covered fraction; report latency before/after, determinism (exact-repro), and the model round-trip removed."`
	Amortize      AmortizeCmd      `cmd:"" help:"Price the authored artifact: how many served hits repay the one-time authoring round-trip, and the re-author cadence at which drift churn erases the win."`
	Hook          HookCmd          `cmd:"" help:"Live PreToolUse hook: read a Bash tool event on stdin, inject the deterministic command category as additionalContext (0 model calls), and demote-on-drift via a windowed state file. The batch→live gap closed."`
	HookDemo      HookDemoCmd      `cmd:"" help:"Drive the real 'crystal hook' binary over a live stream of PreToolUse events (separate processes, on-disk drift window): serve real commands, inject the container-drift class, watch the tier demote live."`
	HookLoop      HookLoopCmd      `cmd:"" help:"Close the loop LIVE: author→serve→demote→re-author→swap artifact→re-promote→resume, across real hook processes. Wires demote-on-drift to the re-author the panel found disconnected; fixes terminal demotion."`
	LocalProbe    LocalProbeCmd    `cmd:"" help:"A5 probe: can a LOCAL small model (ollama) do the categorize chore well/fast enough to be the cheap tier — and the live oracle hook-loop lacks? Accuracy + latency vs Haiku on the covered fraction (det = ground truth)."`
	Guard         GuardCmd         `cmd:"" help:"Reflexive rung: the first CONSTRAINT-type crystallization. A PreToolUse hook that denies 'git add -A|.|--all' (a rule the sweep found re-encoded in 4 projects) and self-monitors via override frequency — its own sub-hybrid-loop."`
	Dispatch      DispatchCmd      `cmd:"" help:"The scaling architecture: ONE PreToolUse hook over a rule LIBRARY (rules as data + named tested matchers), evaluated in-process. Subsumes guard; the fix for one-fork-per-rule at hundreds/thousands of rules. Per-rule self-monitoring state."`
	PublicaiProbe PublicAIProbeCmd `cmd:"" name:"publicai-probe" help:"Smoke-test the Public AI Gateway tier (open/sovereign models, OpenAI-compatible): one cheap chat completion confirms key, gateway reachability, response decode, and disk cache. A cloud-cheap OPEN-model rung between Haiku and local ollama."`
	Viz           VizCmd           `cmd:"" help:"Serve the live flow dashboard (docs/viz/live.html) over HTTP so a second monitor can watch requests shift left across the menu in realtime; re-rendered from the real counts a hook-loop run emits."`
	Library       LibraryCmd       `cmd:"" help:"Serve crystallized artifacts from a LIBRARY behind a confidence + cooldown gate, modeled on groupchat's deployed meme system: match a context against entries (deploy_when/too_much_if/cooldown metadata), serve the best match when confident and not over-firing, ABSTAIN otherwise (a wrong artifact is worse than no artifact). Deterministic serve (0 model) = the cheap tier; the matched artifact is the recipe a weaker executor runs. The serve layer of the recipe ladder + the rule-library dispatch the README roadmaps."`
	LibraryHook   LibraryHookCmd   `cmd:"" name:"library-hook" help:"The serve tier wired to REAL hook events (batch→live for serve): read a Claude Code hook event on stdin and inject the matched crystallized recipe as additionalContext (0 model calls), behind the confidence + cooldown + too-much gate, with cooldown and demotion persisting across the fresh-process-per-event boundary via a state file. The library's entries are keyed on INTENT, so the rich surface is UserPromptSubmit (the prompt prose); PreToolUse works too but exposes only tool-observable entries (the git-add guard). Fail-open. --demote/--promote NAME are the re-author loop's serve-layer control ops."`
	LibraryHookDemo LibraryHookDemoCmd `cmd:"" name:"library-hookdemo" help:"Drive the real 'crystal library-hook' binary over a live stream of hook events (separate processes, on-disk cooldown/demotion window): serve on a strong match, abstain the repeated intent on cooldown ACROSS the process boundary, and demote an entry live so it stops serving in every subsequent process. Proves the serve tier is live, not batch."`
	PlanShift     PlanShiftCmd     `cmd:"" name:"planshift" help:"The recipe-ladder transfer harness on a REAL code-change chore (reconstruct a kong subcommand from its contract), with the plan rung GATED by plancheck. Tests whether a verified plan beats the ungated plan that scored WORSE than no-recipe (43% vs 57%) in 'crystal transfer'. Three arms — none / plan-ungated / plan-gated (Opus authors K plan styles, plancheck's forecast.pClean minus a co-mod-gap penalty picks the best; abstain if none clears the bar). Verifier = the produced scaffold's enum/slice/flag contract vs the real command (cmdspec, AST-only, no scratch builds). Reports whether pClean separated passing plans from failing — the gate's own validity. Requires the plancheck binary on PATH."`
	Recipe        RecipeCmd        `cmd:"" help:"Lint crystallized-artifact RECIPES against the schema (internal/recipe, docs/RECIPE_SCHEMA.md): a recipe is a typed, diffable object, so it is checked like code. Reports each recipe's rung (actuator expressivity), executor (the weakest tier that can run it = model-burden floor), verifier (the machinery carrying correctness), and narrow-waist degrees-of-freedom; ENFORCES the coupling (a 'code' rung that still needs a model, or is gated by a model-judge, is rejected). Emits structured rows rendered as a ladder per intent — the artifact-schema half of the recipe ladder, channeling Odendahl's generalization-shaping framing."`
	Transfer      TransferCmd      `cmd:"" help:"MEASURE the recipe ladder (docs/RECIPE_LADDER.md) instead of asserting it: sweep recipe-rung × executor-tier and report the TRANSFER FRACTION — how often a weak model (Haiku) following Opus's recipe reproduces the golden — at each rung (none/plan/recipe/pseudocode/code). Tests the claim that recipe-specificity sets the executor floor: does the weak model take more of the chore as the recipe sharpens? Opus-direct is the ceiling; the deterministic code rung needs no model; local-open is the deferred cell."`
	AuthorFn      AuthorFnCmd      `cmd:"" name:"author-fn" help:"The defn-backed Authorer: the first END-TO-END sediment of a real function. Selects a test-covered, low-blast-radius definition (defn impact), has Opus reimplement it BLIND to the tests, gates the candidate on its covering tests scoped to the package (always restoring the working tree), and runs a NEGATIVE CONTROL (a compiling-but-wrong impl must be rejected — proving the covering tests are load-bearing). A passing candidate is PROPOSED, never auto-applied. Requires defn ingest first."`
	Sediment      SedimentCmd      `cmd:"" help:"The defn-backed Verifier stage over a repo's OWN arbitrary Go definitions: uses defn for per-definition test COVERAGE (the Confidence gate) and scoped affected-test execution, reporting the cheaply-verifiable FRACTION (the master variable) — which definitions a cheap replacement can be gated for (sediment-ready) vs which must stay frontier (unverifiable, refused). Turns 'arbitrary Go logic sediments' from proven-in-miniature into a running claim. Requires defn ingest first."`
	Discover      DiscoverCmd      `cmd:"" help:"The watch-don't-ask front half of the codegen loop: scan a code corpus, find the DOMINANT recurring command shape by STRUCTURE (help-tagged flag fields, not the '*Cmd' name), and report whether it clears the recurrence bar to be a crystallization candidate. Turns dogfood's hand-pointed corpus into a found chore; the codegen sibling of sweep/measure."`
	Dogfood       DogfoodCmd       `cmd:"" help:"Crystallize crystal's OWN recurring chore (the kong subcommand scaffold) as a real, non-toy test of the framing: an operational golden-free verifier (go build + kong registration + a behavioral contract) instead of a pre-known golden, and a miss discovered by serving the irregular commands (enum flags, repeatable slices) the generator was never shown. Emits a coverage table marking the measured g<1 boundary — where build+register pass a behaviorally-wrong scaffold. Generated scaffolds are throwaway proposals; the parser+verifier+loop are the keepable tooling."`
	Demo          DemoCmd          `cmd:"" help:"The compelling first run: build an N-unit code package in one session and watch the apportionment of work slide from all-frontier (Opus produces every unit) to mostly-deterministic (Opus authors a generator once, it runs free behind a golden verifier), with a seeded irregular entity that demotes, re-authors, and recovers. Live by default (real Opus latencies, disk-cached); --offline runs the whole staircase key-free for CI/screenshots."`
	Sweep         SweepCmd         `cmd:"" help:"The autonomous DISCOVERY front-end (the vision's first verb). Default: CONSTRAINTS — rules RE-ENCODED across N projects' docs, clustered by command signature. --procedures: recurring multi-command SEQUENCES from session transcripts (the cupel release-dance pattern; add --novel to skip generic git churn). Deterministic, no model."`
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
