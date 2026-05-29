# crystal

**Shift mechanical work off the frontier model onto faster, cheaper, eventually-deterministic
tiers — and keep it there as patterns drift, without silent quality loss.**

## Why bother

The frontier model is the bottleneck on axes that *don't* go away as prices fall: **latency**
(round-trip + big-model decode, compounding across call-hungry agentic loops), **throughput and
ratelimit headroom** (finite frontier budget; mechanical work crowds out the cognitive core), and
**determinism / dependency** (sampling variance, API dependency, data egress). Moving the
mechanical, repetitive work *left* — onto a smaller/local, ultimately deterministic tier — wins all
of those, and runs on hardware you own. (The win is **not** token cost: frontier prices are
collapsing, so "save tokens" is the wrong frame — it's latency, determinism, sovereignty, and
throughput.)

But cheap tiers are *worse*, so shifting left naively trades a quality collapse you won't notice for
the speed. Two things make it actually bankable:

- **A verifier gate** turns "cheap tier, risky" into "cheap tier, gated" — the work only migrates
  down if a check confirms it reproduces the frontier outcome, and it's **demoted the moment it
  drifts**. This is the difference between a real win and silent rot.
- **Self-authoring** makes it scale and *survive*: the frontier tier writes the cheaper tier's
  harness (prompt, schema, gate, verifier) and **re-writes it on drift**, so you don't hand-engineer
  a bespoke migration per task and watch it decay. A static crystallized hook rots; a self-authored
  one adapts.

So the useful claim is **automatic, drift-surviving shift-left at held quality.** The hard part —
and crystal's actual contribution — is the discipline that keeps it safe when the loops stack and
run unattended.

## What crystal is

A **trust substrate for recursive self-authoring loops**: verifier-gated promotion,
drift-triggered demotion, a tamper-proof kernel (a verifier the authored tier structurally cannot
rewrite — the [DGM](docs/THESIS.md) antidote), and **instrumented per-hop signal loss (λ)**. These
primitives are *edge-local and node-local* — a verifier gates a promotion **edge**, λ is
per-**edge** loss, the kernel is a **node** property — so the discipline composes over an arbitrary
graph of loops, not just a line: a vertical **stack** of tiers (the simplest case), but also
**trees** (one supervisor authoring many parallel sub-harnesses), **dev-time cycles** (a critic loop
wrapping a runtime loop), and **meshes** (co-equal loops generating each other's surface). The
self-authoring *mechanism* is now commoditizing (AutoHarness, STOP, SICA, Gödel Agent — see
[`docs/PRIOR_ART.md`](docs/PRIOR_ART.md)); making it *safe to run unattended* is the open problem.

(The experiments below exercise only the vertical/linear case; trees, cycles, and meshes are in
scope but not yet measured.) Full framing in [`docs/THESIS.md`](docs/THESIS.md); original charter in
[`PROJECT_BRIEF.md`](PROJECT_BRIEF.md).

> Status: research-stage personal project. The eval/promote/demote gate, the drift detector, the
> topology sim, and four live grounding experiments are built and run. The headline contribution —
> a tamper-proof recursive guardrail demo — is **not yet built**. Numbers in the findings docs are
> the ground truth; this README summarizes.

## What's been measured

The two knobs that decide whether shift-left is safe are **g** (does a verifier catch the cheap
tier's errors — *which work is safe to migrate down*) and **λ** (does the supervisory signal survive
relay — *how deep the supervision reaches before it goes blind*). Four live experiments grounded
both on real and constructed substrates with hard, by-construction labels.

| experiment | question | result |
|---|---|---|
| `ground-hop` | g on byte-exact tool drift | **g = 1.00**; per-hop λ ≈ 0 (typed channel) |
| `uncover-hop` | g when a check provably can't catch the error | schema **0.00** / substring-grounded **0.50**; a content-bearing prose channel recovered the residual at λ ≈ 0 |
| `depth-sweep` | does loss compound over relay depth? | error **detection** flat at 1.00 through depth 6 |
| `content-sweep` | does *corrective content* compound-lose? | content fidelity **flat at ~0.70** through depth 6 — **no compounding** |

**Central finding:** the loss is at **hop 1, not in depth and not predicted by format.** A
content-bearing prose channel is ~as lossless as a typed one at one hop (the real axis is
*content-vs-verdict*, not prose-vs-typed); neither detection nor content fidelity compounds-loses
through six relays; the binding constraint is the first hop's summarizer/reader quality (~70% here,
lowest on name-distractor errors). This *tensions with* the lattice's depth-pessimism — see the
caveats in the findings docs (cooperative relay, N=14, depth 6).

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
--help` for the full list.

## Reading order (for the details)

1. [`docs/THESIS.md`](docs/THESIS.md) — framing (binary → ladder → recursive → trust substrate) and honest SOTA positioning. [`PROJECT_BRIEF.md`](PROJECT_BRIEF.md) is the original charter.
2. [`docs/PRIOR_ART.md`](docs/PRIOR_ART.md) — verified citation map: which primitives are prior art, which seams are open.
3. [`docs/SUBSTRATE_SURVEY.md`](docs/SUBSTRATE_SURVEY.md) — the real transcript schema the eval gate replays against.
4. **Phase-1 gate:** [`MEASURE_FINDINGS.md`](docs/MEASURE_FINDINGS.md), [`DRIFT_FINDINGS.md`](docs/DRIFT_FINDINGS.md), [`LATTICE_FINDINGS.md`](docs/LATTICE_FINDINGS.md).
5. **The grounding arc, in order:** [`EXPERIMENT_FINDINGS.md`](docs/EXPERIMENT_FINDINGS.md) → [`GROUNDHOP_FINDINGS.md`](docs/GROUNDHOP_FINDINGS.md) → [`UNCOVERHOP_FINDINGS.md`](docs/UNCOVERHOP_FINDINGS.md) → [`DEPTHSWEEP_FINDINGS.md`](docs/DEPTHSWEEP_FINDINGS.md) → [`CONTENTSWEEP_FINDINGS.md`](docs/CONTENTSWEEP_FINDINGS.md).

## Why the numbers are trustworthy

Every aggregate is checked against raw per-item output before it's called a finding (`--verbose`
dumps, pre-registered caveats, hard by-construction labels). This isn't ceremony: five times a
fluent, confident number dissolved on inspection — the 219 walker miscount, the lattice depth
artifacts, `experiment`'s λ=0.90, `ground-hop` run-1's λ=0, and `depth-sweep`'s "content erodes
with depth" (a self-correction, overturned by `content-sweep`). **Read the `--verbose` output
before trusting any aggregate.**

## Hard rules (from the brief, still binding)

- **No verifier, no crystallization.** An unregistered tool/channel is *unverifiable*, never a silent pass.
- **Fail loud.** Every divergence is localized; empty/ambiguous verdicts are surfaced, never defaulted.
- **Stable kernel.** A tier may author the harness *below* it; it must not be able to rewrite the gate *above* it (the [DGM](docs/THESIS.md) antidote).
- **Demote more aggressively than you promote**, and never demote judgment to a tier that can't carry it.
