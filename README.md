# crystal

**A trust substrate for recursive self-authoring LLM stacks.**

When a capable model authors the harness a cheaper model runs inside — and that cheaper model in
turn authors a harness for a cheaper one, with feedback flowing up (drift, escalation) and down
(spec, verifier) — you get a *stack of self-improving tiers*. The mechanism (a model writing the
cage another model runs in) is now commoditizing (AutoHarness, STOP, SICA, Gödel Agent — see
[`docs/PRIOR_ART.md`](docs/PRIOR_ART.md)). What is **not** solved is making such a stack **safe to
run unattended**: catching a lower tier's drift through a degraded up-signal and re-authoring the
harness below it, without a human, without silent degradation, behind a guardrail the lower tier
**cannot rewrite**.

Crystal is the discipline for that: verifier-gated promotion, drift-triggered demotion, a
tamper-proof kernel, and **instrumented per-hop signal loss**. Full framing in
[`docs/THESIS.md`](docs/THESIS.md); original charter in [`PROJECT_BRIEF.md`](PROJECT_BRIEF.md).

> Status: research-stage personal project. The eval/promote/demote gate, the drift detector, the
> topology sim, and four live grounding experiments are built and run. The headline contribution —
> a tamper-proof recursive guardrail demo — is **not yet built**. Numbers in the findings docs are
> the ground truth; this README summarizes.

## The epistemic posture (read this first)

Crystal's defining habit is **catching its own manufactured confidence.** Five times now a fluent,
specific, confident number or narrative dissolved when checked against raw output:

1. `measure` → a "219 deacon heartbeat" crystallizable pattern (a walker miscount).
2. `lattice` → a "max safe depth 2 / depth 30" (gain-contingent + a search-cap artifact).
3. `experiment` → "Haiku beats Opus, λ=0.90" (exact-match gold + a self-graded summary channel).
4. `ground-hop` run 1 → "λ_prose=0.00" (a 5-token budget starved the verdict to empty text).
5. `depth-sweep` → "content erodes with depth" (misread depth-1 errors as depth-progression;
   the content-fidelity sweep showed it flat).

Each was caught only by a verifier checking the claim against ground truth — and #5 corrected a
*prior crystal finding*. **That the project keeps catching its own fluent-but-wrong output is the
strongest live evidence that the trust discipline is necessary — the thesis demonstrated on
itself.** The standing rule, enforced in every experiment: *no number is a finding until something
has checked it against the raw source* (hence `--verbose` per-item dumps and pre-registered
caveats).

## What's been measured (the experiment arc)

Four live experiments grounded the lattice's two assumed knobs — guardrail coverage **g** and
per-hop signal loss **λ** — on real and constructed substrates with hard, by-construction labels.

| experiment | question | result |
|---|---|---|
| `ground-hop` | g on byte-exact tool drift | **g = 1.00**; per-hop λ ≈ 0 (typed channel) |
| `uncover-hop` | g when a check provably can't catch the error | schema **0.00** / substring-grounded **0.50**; a content-bearing prose channel recovered the residual at λ ≈ 0 |
| `depth-sweep` | does loss compound over relay depth? | error **detection** flat at 1.00 through depth 6 |
| `content-sweep` | does *corrective content* compound-lose? | content fidelity **flat at ~0.70** through depth 6 — **no compounding** |

**Central finding (corrected):** the loss is at **hop 1, not in depth and not predicted by
format.** A content-bearing prose channel is ~as lossless as a typed one at one hop (the real axis
is *content-vs-verdict*, not prose-vs-typed); neither detection nor content fidelity compounds-loses
through six relays; the binding constraint is the first hop's summarizer/reader quality (~70% here,
lowest on name-distractor errors). This *tensions with* the lattice's depth-pessimism — see the
caveats in the findings docs (cooperative relay, N=14, depth 6).

## Guided reading order

1. [`PROJECT_BRIEF.md`](PROJECT_BRIEF.md) — original charter and hard rules (no-verifier-no-crystallization; fail loud; stable kernel; demote aggressively).
2. [`docs/THESIS.md`](docs/THESIS.md) — how the framing evolved (binary → ladder → recursive → trust substrate) and the honest SOTA positioning.
3. [`docs/PRIOR_ART.md`](docs/PRIOR_ART.md) — verified citation map; which primitives are prior art and which seams are open.
4. [`docs/SUBSTRATE_SURVEY.md`](docs/SUBSTRATE_SURVEY.md) — the real transcript schema the eval gate replays against.
5. **Phase-1 gate findings:** [`MEASURE_FINDINGS.md`](docs/MEASURE_FINDINGS.md), [`DRIFT_FINDINGS.md`](docs/DRIFT_FINDINGS.md), [`LATTICE_FINDINGS.md`](docs/LATTICE_FINDINGS.md) — clustering, the windowed demotion rule, and the topology sim (with two corrections).
6. **The live grounding arc, in order:** [`EXPERIMENT_FINDINGS.md`](docs/EXPERIMENT_FINDINGS.md) (instrument-invalid, diagnosed) → [`GROUNDHOP_FINDINGS.md`](docs/GROUNDHOP_FINDINGS.md) → [`UNCOVERHOP_FINDINGS.md`](docs/UNCOVERHOP_FINDINGS.md) → [`DEPTHSWEEP_FINDINGS.md`](docs/DEPTHSWEEP_FINDINGS.md) (carries a correction banner) → [`CONTENTSWEEP_FINDINGS.md`](docs/CONTENTSWEEP_FINDINGS.md).

## Building and running

Go 1.26, [kong](https://github.com/alecthomas/kong) CLI. Live experiments need `ANTHROPIC_API_KEY`
in `.env` (or the environment); every LLM call is disk-cached by content hash, so re-runs are free.

```sh
go build ./...        # or: go run . <subcommand>
go test ./...         # internal/eval/eval_test.go is the Phase-1 go/no-go gate

go run . ground-hop   --verbose    # g and per-hop λ on real byte-exact drift
go run . uncover-hop  --verbose    # g<1 regime + fuzzy recovery of the residual
go run . depth-sweep  --verbose    # detection recall vs relay depth
go run . content-sweep --verbose   # corrective-content fidelity vs relay depth
```

Offline subcommands (no API): `extract` (build the redacted corpus), `eval` (replay a synthetic
artifact), `measure`, `drift`, `lattice`. `probe` is a one-call plumbing check. Run `go run .
--help` for the full list. **Always read the `--verbose` per-item output before trusting any
aggregate** — that is the rule the five catches earned.

## Hard rules (from the brief, still binding)

- **No verifier, no crystallization.** An unregistered tool/channel is *unverifiable*, never a silent pass.
- **Fail loud.** Every divergence is localized; empty/ambiguous verdicts are surfaced, never defaulted.
- **Stable kernel.** A tier may author the harness *below* it; it must not be able to rewrite the gate *above* it (the [DGM](docs/THESIS.md) antidote).
- **Demote more aggressively than you promote**, and never demote judgment to a tier that can't carry it.
