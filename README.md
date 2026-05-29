# crystal

**Shift mechanical work off the frontier model onto faster, cheaper, eventually-deterministic
tiers — and keep it there as patterns drift, with degradation that's loud instead of silent.**

## Why bother

The frontier model is the bottleneck on axes that *don't* go away as token prices fall:

- **Determinism** — a crystallized deterministic hook is exactly reproducible; no sampling variance. The most durable win, and the one a cloud model structurally can't give you.
- **Sovereignty** — the cheapest tier can run on hardware you own, with no API dependency or data egress. (Partial, honestly: see the caveat below.)
- **Latency** — a local/cheap tier skips the frontier round-trip and big-model decode; compounds across call-hungry agentic loops. *Conditional* on the gate being deterministic (no added round-trip) and authoring amortizing over many hits.
- **Throughput / ratelimit headroom** — frees finite frontier budget for the cognitive core. Real today, but a transient capacity artifact that fades as frontier supply grows.

The win is explicitly **not** token cost — frontier prices are collapsing, so "save tokens" is the wrong frame. It's the bundle above, weighted toward determinism and sovereignty.

**Two honest caveats up front.** *Sovereignty is partial:* the cheap tier is sovereign at
steady-state inference, but the self-authoring/re-authoring step still touches the frontier, so
it's "sovereign at inference, frontier-dependent at adaptation time." And the **deterministic-tier**
case (a static hook) is real and built; the **local-small-model** tier (RTX 3080 + LoRA, per the
brief) is **aspirational and unmeasured** — every live experiment here uses cloud Haiku as the cheap
tier.

### What makes it bankable (and what doesn't yet)

Cheap tiers are *worse*, so shifting left naively trades a quality collapse you won't notice for the
speed. Two mechanisms hold the line:

- **A verifier gate** (built) — work migrates down only if a check confirms it reproduces the
  frontier outcome, and is **demoted when it drifts in a way the verifier can express.** This is the
  difference between a real win and silent rot. *Limit:* a deterministic check can't see semantic
  drift it can't express (see `g` below), and a self-improving tier will game a verifier it can
  reach — the [DGM](docs/THESIS.md) result. The **tamper-proof** version of the gate is **not yet
  built**, so today's gate is the gameable kind for anything beyond a fixed deterministic hook.
- **Self-authoring** (the adaptive part) — the frontier tier writes the cheap tier's harness and
  re-writes it on drift, so a migration doesn't rot the way a hand-written static replacement does.

**The sharper move is to decompose, not just downshift.** Don't hand a cheap model the *whole*
task; hand it the small fuzzy gap and let it drive a robust deterministic tool for the rest. A
cheap model + `grep` to verify a quote beats a frontier model doing it from scratch — cheaper,
faster, and the tool's output is trivially checkable (so it gate-able). Your cost is set by the
fraction you *can't* hand to a tool: offload the mechanical, high-coverage sub-steps (g≈1) to the
ecosystem's battle-tested wheels, and pay model intelligence only for the irreducible
judgment residual (1−g). The honest limit, proven by our own measured leak below: **semantic
judgment doesn't offload** — a tool helps where the hard part is mechanical, not where it's "which
entity is the subject."

This is deliberately a **cheap lunch, not a free one**: perfect isn't the goal; scrappy, organic,
and *evolving* is — reuse robust tools instead of reinventing them, accept the residual leak, and
let the system accrete which decompositions hold (and demote the ones that drift). Unlike per-request
model routers (Copilot/Cursor "Auto", RouteLLM — see [`docs/PRIOR_ART.md`](docs/PRIOR_ART.md)), which
re-decide cold every prompt, crystal's policy is **per-recurring-chore and stateful** — that's the
niche, and the evolution, that nobody else occupies.

The deeper frame (a hypothesis, in [`docs/THESIS.md`](docs/THESIS.md)): the master variable is the
*cheaply-verifiable fraction*, not the model tier — checking is cheaper than generating, so you
place work as cheaply as you can *verify* it. Crystallizing a recurring chore is **partial
evaluation** of it; verification-shaped tasks decompose best, judgment-shaped tasks resist.

So the honest claim is **automatic, drift-surviving shift-left with loud degradation** — *held
quality* on the verifier-covered (and tool-covered) fraction, *detection + demotion* (not guaranteed
reproduction) on the uncovered residual.

## Try it (on your own data)

`crystallize` is the humble shift-left, end to end, no LLM in the loop: it scans your Claude Code
transcripts, finds the dominant repetitive deterministic command, proposes a static hook,
promote-gates it on determinism, serves a holdout while watching for drift, demotes if it drifts,
and writes a redacted deployable artifact.

```sh
go run . crystallize --home ~ --match "git status"
# discover → propose (modal hook) → gate (PROMOTE/REFUSE) → serve+drift-monitor → demote
# → writes crystallized/<pattern>.json   (refuses loudly if the command isn't deterministic enough)
```

This is the v0 (deterministic-tier) slice; the LLM/local-model tiers are the roadmap
([`docs/ROADMAP.md`](docs/ROADMAP.md)).

## What's been measured

Two knobs decide whether shift-left is safe: **g** (does a verifier catch the cheap tier's errors —
*which work is safe to migrate down*) and **λ** (does the supervisory signal survive relay — *how
deep supervision reaches before going blind*). Four experiments grounded them with hard,
by-construction labels. **Only `ground-hop` runs on real transcript records; the other three share
one 14-item hand-authored synthetic corpus**, so the content/depth conclusions rest on the
constructed side.

| experiment | question | result |
|---|---|---|
| `ground-hop` | g on byte-exact tool drift (real records) | **g = 1.00**; per-hop λ ≈ 0 (typed channel; ≈0 is tautological for byte-exact) |
| `uncover-hop` | g when a check provably can't catch the error | schema **0.00**; substring-grounding cleanly separates the two constructed drift classes (catches absent-value, misses in-source distractor) — the in-source half is uncatchable by *any* substring check |
| `depth-sweep` | does loss compound over relay depth? | error **detection** flat at 1.00 through depth 6 |
| `content-sweep` | does *corrective content* compound-lose? | content fidelity **flat at ~0.70** through depth 6 — no compounding |

**Central finding (scoped):** under a *cooperative, instructed relay (N=14, depth 6, Haiku)*, the
loss is at **hop 1, not in depth.** A content-bearing channel is ~as lossless as a typed one at one
hop (*consistent with* "content-vs-verdict, not prose-vs-typed" being the axis — though that's a
cross-experiment comparison, not a same-item A/B). The ~0.70 ceiling is an **entangled reading**:
it mixes channel loss with a fallible recovery-reader (a single Opus call that sometimes picks the
wrong span even when the channel carried the right answer) and is **not** cleanly a channel
property. See the findings docs for the confound and the wide-CI caveats — *do not quote ~0.70 or
λ≈0 as constants.*

**The payoff itself** (`payoff`, the first value-prop measurement, not just the safety discipline):
shifting a chore down the cost gradient (Opus→Sonnet→Haiku) behind a deterministic gate. Haiku saved
**~46% median latency** at *mostly*-held quality (0.86 vs Opus 0.93) — the latency win is real and
large, but the deterministic gate leaks the cheap tier's in-source semantic errors, so "held
quality" is conditional on a gate that covers the error mode (an LLM gate would, but erases the
latency win). And shift-left isn't binary: on this chore the gradient was *bimodal* (Opus 0.93, or
0.86 at either cheaper tier — Sonnet bought nothing over Haiku), so the useful knee is
chore-dependent. The honest shift-left is a tradeoff, not a free lunch. See
[`PAYOFF_FINDINGS.md`](docs/PAYOFF_FINDINGS.md).

## The safety discipline (secondary)

Shift-left is the point; this is what keeps it from rotting once loops stack. The verifier gate and
drift detector (built) are the load-bearing parts. The ambitious extension — a **tamper-proof
kernel** the authored tier structurally cannot rewrite, **per-hop λ as a live oversight altimeter**,
and composition over arbitrary loop topologies (trees, dev-time cycles, meshes — *in scope, not yet
measured*; only the linear case is exercised) — is **mostly unbuilt** and has substantial prior art
(AI Control, reward-tampering; see [`docs/PRIOR_ART.md`](docs/PRIOR_ART.md)). The defensible open
sub-problem is narrow: a tamper-proof kernel *across a self-authoring/drift loop*. Full framing and
honest positioning in [`docs/THESIS.md`](docs/THESIS.md); original charter in
[`PROJECT_BRIEF.md`](PROJECT_BRIEF.md).

> Status: research-stage personal project. Built and tested: the eval/promote/demote gate, the drift
> detector, the topology sim, the `crystallize` lifecycle, and four grounding experiments. Not built:
> the tamper-proof kernel, the LLM/local-model tiers, anything running unattended over time. The
> findings docs are ground truth; this README summarizes.

## Reading order (for the details)

1. [`docs/THESIS.md`](docs/THESIS.md) — how the framing evolved and honest SOTA positioning. [`PROJECT_BRIEF.md`](PROJECT_BRIEF.md) is the original charter.
2. [`docs/ROADMAP.md`](docs/ROADMAP.md) — what's built, what's next, and the vertical slice that would prove the thesis.
3. [`docs/PRIOR_ART.md`](docs/PRIOR_ART.md) — citation map: which primitives are prior art, which seams survive.
4. [`docs/SUBSTRATE_SURVEY.md`](docs/SUBSTRATE_SURVEY.md) — the real transcript schema the gate replays against.
5. **Phase-1 gate:** [`MEASURE_FINDINGS.md`](docs/MEASURE_FINDINGS.md), [`DRIFT_FINDINGS.md`](docs/DRIFT_FINDINGS.md), [`LATTICE_FINDINGS.md`](docs/LATTICE_FINDINGS.md).
6. **The grounding arc, in order:** [`EXPERIMENT_FINDINGS.md`](docs/EXPERIMENT_FINDINGS.md) → [`GROUNDHOP_FINDINGS.md`](docs/GROUNDHOP_FINDINGS.md) → [`UNCOVERHOP_FINDINGS.md`](docs/UNCOVERHOP_FINDINGS.md) → [`DEPTHSWEEP_FINDINGS.md`](docs/DEPTHSWEEP_FINDINGS.md) → [`CONTENTSWEEP_FINDINGS.md`](docs/CONTENTSWEEP_FINDINGS.md) → [`PAYOFF_FINDINGS.md`](docs/PAYOFF_FINDINGS.md) (the value-prop measurement).

## Building and running

Go 1.26, [kong](https://github.com/alecthomas/kong) CLI. Live experiments need `ANTHROPIC_API_KEY`
in `.env` (or the environment); every LLM call is disk-cached by content hash, so re-runs are free.

```sh
go build ./...        # or: go run . <subcommand>
go test ./...         # internal/eval/eval_test.go is the Phase-1 go/no-go gate

go run . triage --verbose                            # v1 slice: map-reduce + verifier on a real chore (your Bash usage), 0 frontier calls
go run . crystallize --home ~ --match "git status"   # the shift-left lifecycle on your own data
go run . payoff       --verbose    # the value prop: latency saved vs quality held, Opus→Haiku behind a gate
go run . ground-hop   --verbose    # g and per-hop λ on real byte-exact drift
go run . uncover-hop  --verbose    # the g<1 regime + fuzzy recovery of the residual
go run . depth-sweep  --verbose    # detection recall vs relay depth
go run . content-sweep --verbose   # corrective-content fidelity vs relay depth
```

Other offline subcommands (no API): `extract`, `eval`, `measure`, `drift`, `lattice`. `probe` is a
one-call plumbing check. `go run . --help` for the full list.

## Why the numbers are trustworthy

Every aggregate is checked against raw per-item output before it's called a finding (`--verbose`
dumps, pre-registered caveats, hard by-construction labels). This isn't ceremony: five times a
fluent, confident number dissolved on inspection — the 219 walker miscount, the lattice depth
artifacts, `experiment`'s λ=0.90, `ground-hop` run-1's λ=0, and `depth-sweep`'s "content erodes with
depth" (a self-correction, overturned by `content-sweep`). That track record is a *measurement*
discipline — good engineering hygiene with `--verbose` — not evidence for the unbuilt tamper-proof
kernel; don't conflate the two. **Read the `--verbose` output before trusting any aggregate.**

## Hard rules (from the brief, still binding)

- **No verifier, no crystallization.** An unregistered tool/channel is *unverifiable*, never a silent pass.
- **Fail loud.** Every divergence is localized; empty/ambiguous verdicts are surfaced, never defaulted.
- **Stable kernel.** A tier may author the harness *below* it; it must not be able to rewrite the gate *above* it (the [DGM](docs/THESIS.md) antidote — aspirational; today's gate isn't yet tamper-proof).
- **Demote more aggressively than you promote**, and never demote judgment to a tier that can't carry it.
