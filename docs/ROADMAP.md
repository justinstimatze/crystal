# Roadmap

Priority order reflects the project's actual interest: **the humble shift-left is the point.** The
trust substrate is the safety scaffolding that keeps shift-left from rotting — necessary, but
secondary, and heavily prior-arted ([`PRIOR_ART.md`](PRIOR_ART.md)). Milestones are "done when
*measured*," not "done when it exists."

## Where it is now

**Built + tested + run:**
- Eval/promote/demote gate (`internal/eval`, `internal/compare`) — per-tool typed comparators, the Phase-1 go/no-go test.
- Drift detector (`internal/drift`) — windowed M-in-W demotion (consecutive-K is evadable; see `DRIFT_FINDINGS.md`).
- `crystallize` lifecycle — discover → propose modal hook → promote-gate → serve+drift-monitor → demote → emit redacted artifact, on real transcripts. **This is the v0 deterministic-tier shift-left, end to end.**
- Topology sim (`internal/lattice`) — algebra, not emergent; corrected twice.
- Four grounding experiments — `g` and `λ` measured (one on real records, three on a 14-item synthetic corpus).

**Not built (the honest gaps):**
- The **payoff is unmeasured.** Every result so far measures the *safety discipline* (does the gate catch errors, does the signal survive). Nothing yet shows shift-left *bought* anything — no end-to-end latency/throughput before-vs-after.
- The crystallized artifact is written to disk but **not actually installed/served** as a live hook.
- The **LLM cheap tier** and the **local-model tier** (RTX 3080 + small model + LoRA, per the brief) — every experiment uses cloud Haiku.
- The **tamper-proof kernel** — today's gate is the gameable kind (the DGM result).
- Anything running **unattended over real time**; any topology past the linear relay.

## Track A — shift-left (primary)

The goal is to turn the value prop from a hypothesis into a measured result, cheapest first.

1. **Serve the deterministic hook for real.** Install a `crystallize`-emitted artifact as a live
   PreToolUse hook so the static tier actually answers in place of the frontier call. *Proves:* the
   loop closes end to end on real use. *Done when:* a real repetitive command is served locally and
   the gate demotes it on a deliberately introduced drift, live.
2. **Measure the payoff (the breakeven the value prop asserts but never shows).** For a served
   deterministic hook: p50/p99 latency before vs after, determinism (exact-repro rate), and the
   amortization point — how many hits before authoring cost is repaid, and the re-author frequency
   that erases the win. *Proves:* shift-left nets positive on its claimed axes, or finds where it
   doesn't. *Done when:* a before/after table exists for ≥1 real chore.
3. **The LLM-tier slice — FIRST RESULT LANDED** (`payoff`, `PAYOFF_FINDINGS.md`). Shifted a
   mechanical chore Opus→Haiku behind a deterministic gate: **~46% median latency saved**, but
   quality only *mostly* held (0.86 vs 0.93; the deterministic gate leaks in-source semantic errors
   it can't see). The breakeven is demonstrated: deterministic gate = fast + leaky; LLM gate =
   correct + no latency win. *Still to do:* run on *real* (non-synthetic) chores and a real
   agentic-loop baseline; multi-sample latency; an honest LLM-gate latency measurement (not just the
   qualitative claim).
4. **Close the sovereignty gap.** Swap the cheap tier to a local small model (+ LoRA) on owned
   hardware; re-measure latency and held quality. *Proves:* the sovereignty/determinism pitch is
   real, not aspirational. *Done when:* a chore is served from local hardware with the gate intact.

## Track B — trust substrate (secondary, ambitious)

Pursue only as far as it serves Track A's "doesn't rot unattended" requirement. Acknowledge the
prior art (AI Control, reward-tampering) and scope the claim to the narrow survivor: a tamper-proof
kernel *across a self-authoring/drift loop*.

1. **Tamper-proof kernel demo (the would-be headline).** A self-improving tier tries to game its
   own evaluation (the DGM behavior) and is structurally blocked + demoted live, with a visible
   trust readout. *Proves:* the one differentiator that isn't commoditizing. *Done when:* a
   red-team tier provably cannot edit the gate above it and is caught attempting to.
2. **Oversight altimeter.** Surface live per-edge `λ` and current safe depth as a readout. *Proves:*
   the measurement contribution platforms are least likely to ship vendor-neutrally. Lower priority
   until the depth result is re-grounded on real (non-synthetic, non-cooperative) relays.

## Cross-cutting: keep measurements honest

- Re-ground the depth/content results on a *non-cooperative* relay and real data (current results
  are cooperative, instructed, N=14, synthetic — the cued/catchable end of semantic drift).
- Disentangle the ~0.70 content figure: separate channel loss from recovery-reader loss (use
  multiple independent readers, or a structured channel) before treating it as a channel property.
- Every new number goes through the standing rule: `--verbose` per-item inspection before it's a
  finding (the five catches earned it).
