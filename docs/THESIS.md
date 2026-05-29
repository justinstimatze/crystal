# Crystal — current thesis (north star)

This supersedes the framing in `PROJECT_BRIEF.md` where they differ. The brief
is the original charter; this is what we actually believe after building Phase 1
and pressure-testing it.

## The throughline (how the framing evolved)

1. **Binary (wrong).** Crystal first read as "frontier Opus vs. static deterministic
   code" — crystallize a chore into a hook when its output is constant. The
   `measure` sweep under this frame produced "crystallizability ⟂ value": the only
   deterministic-enough patterns are state-independent constant-output commands
   (heartbeats, kills) that aren't worth offloading. **But those measure numbers are
   retracted — see the correction in `MEASURE_FINDINGS.md` (the headline pattern was
   an artifact).** More importantly, the binary axis was itself the error.

2. **Ladder (rejected).** Next read: a tier router (Opus → Sonnet → Haiku → local →
   deterministic hook), routing each chore to the cheapest tier that passes a
   fidelity gate. Rejected: tier routers are well-trodden, boring, and hard for
   little payoff.

3. **Recursive (the actual thesis).** Crystal is **a loop that constructs loops.**
   Each tier's job is not to answer — it is to *author the harness the tier below
   runs inside, and re-author it based on observation.* Opus writes the code that
   builds Sonnet's harness; Sonnet, inside that harness, builds Haiku's; and so on.
   Feedback flows **both ways**: *down* = spec + scaffolding + verifier; *up* =
   results, drift signals, escalation. The whole vertical lattice of nested,
   mutually-supervising loops is one "crystal."

## What's already proven vs. crystal's delta (prior art: publicrecord)

`/home/justin/Documents/publicrecord` is a working multi-tier LLM pipeline
(`opus`+`sonnet` ingest/verify-authoring, `sonnet` batch-verify loop, `haiku`
copilot query-running; 6,317 findings, 0 quality errors). It predates the hybrid
concept but has the shape baked in. It establishes:

- **Tiers can harness tiers and produce verified output at scale.** (So the
  one-rung "Opus authors Sonnet's cage" is *already demonstrated* — not the risky part.)

What publicrecord does **not** do — and these are exactly crystal's delta:

- Its harnesses are **hand-authored** (a human wrote the scripts, prompts, model
  assignments). Crystal: the tier *above* authors them, dynamically.
- Its failure handling is a **`--max-errors N consecutive` stop** — the exact rule
  we proved evadable by intermittent drift (`DRIFT_FINDINGS.md`). And its feedback
  loop is **closed by a human** reading quality issues and editing scripts.

## The riskiest assumption (what we test)

**Can the upper tier detect a lower tier's drift from a signal that propagated UP
through intermediate tiers (degrading en route) and correctly RE-AUTHOR the lower
tier's harness — no human, no silent degradation?** publicrecord shows the tiers
work; it never shows **the stack fixing itself.** That autonomous self-reauthoring
closure, under lossy up-propagation, is the unproven core and the undercity failure
mode (silent degradation, now multiplied per layer).

## The reusable assets (Phase 1 wasn't wasted)

- **The eval/promote/demote gate is the unit cell of the lattice.** Every layer
  boundary (Opus⊃Sonnet, Sonnet⊃Haiku) *is* a fidelity gate: does the lower tier's
  output match the upper tier's behavior on this chore? Built, tested, tier-agnostic.
- **The drift detector is the up-feedback / ambient-meta-loop** — and we already
  found its consecutive-K rule is evadable; the windowed M-in-W rule is the fix.
  This matters *more* in the stack: drift at level N is invisible to level N−2, so
  trustworthy up-propagation is load-bearing for the whole crystal.

## Test ladder (cheapest riskiest-assumption test first)

1. **Deterministic topology sim** (`internal/lattice`) — stack ≥2 gate-tiers, inject
   bottom drift, add a tunable per-hop information-loss knob (the anti-rigging guard),
   and find the (depth × loss) frontier where self-reauthoring stops converging =
   **max safe stack depth.** Zero API cost; tests a necessary condition before any
   live spend. If even the idealized loop can't converge past depth 2, the live
   version is hopeless and publicrecord's human-in-the-loop was load-bearing.
2. **Live 2-boundary** (later) — real Opus authors a real Sonnet harness; corrupt
   Sonnet; does Opus, fed the propagated signal, re-author a fix? Real API cost.

## Mapping to the hybrid framework

Crystal is hybrid's "dev-time loop wrapping the runtime," made recursive: Opus's
dev-time loop emits Sonnet's runtime harness, whose dev-time loop emits Haiku's.
The ambient-meta-loop (deterministic hook fires a parallel evaluator whose verdict
feeds back) is the per-boundary mechanism, stacked vertically.
