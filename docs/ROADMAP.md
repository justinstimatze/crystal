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
Unifying lens (THESIS "general principle"): every rung is *maximize the cheaply-verifiable fraction*
— place work as cheaply as you can verify it; crystallize = partial-evaluate a recurring chore.

0. **v1 slice SHIPPED** (`triage`, `SLICE_FINDINGS.md`). The full stack end to end on a real chore
   (categorize real Bash usage): deterministic verifier rules cover **g=0.77** and gate the cheap
   model, Haiku does only the 23% residual, deterministic reduce tallies — **0 frontier calls**.
   Shipping on real data found+fixed a verifier bug (compound `cd && action` mislabeled nav) the
   synthetic experiments missed. The stack composes; value-prop no longer only hypothesis. *Caveat:*
   batch-over-corpus, not a live served hook; rules hand-authored, not tier-authored/demoted-on-drift.
0b. **Self-author the verifier — RESULT LANDED** (`author`, `AUTHOR_FINDINGS.md`). The actual crystal
   mechanism, not the hand-written shape: the expensive tier (Opus) authors `triage`'s deterministic
   rule table from labeled examples → a deterministic gate promotes only at ≥0.90 holdout fidelity
   vs the reference → a windowed-M-in-W drift trigger forces a verified re-author. On the live corpus
   (8,589 holdout): authored **0.93 → PROMOTE**, corrupted negative control **0.01 → REJECT** (gate is
   load-bearing, not a rubber stamp), injected container class **DEMOTE at index 2 → re-author →
   8/8 = 1.00 RECOVERED**. Author fidelity scaled with sample size (200→0.87, 400→0.89, 800→0.93) at
   a *fixed* 0.90 bar — every green came from more data, never a lowered threshold; the gate rejected
   plausible-but-imperfect tables (0.87–0.89) rather than serving them. *Caveat:* fidelity-to-reference,
   not ground-truth (the reference is the hand-rules); still a batch, not a live served hook.
1. **Serve the deterministic hook for real.** Install a `crystallize`-emitted artifact (or `triage`'s
   rule set) as a live PreToolUse hook so the static tier actually answers in place of the frontier
   call. *Proves:* the loop closes end to end on live use. *Done when:* a real repetitive command is
   served locally and the gate demotes it on a deliberately introduced drift, live.
2. **Measure the payoff — FIRST RESULT LANDED** (`serve`, `SERVE_FINDINGS.md`). Served the
   deterministic tier in place of the cheap-model call on the covered fraction of the real Bash
   chore: **~7µs/call vs Haiku p50 640ms (~90,000×) at zero quality cost** (the rule IS the reference
   on what it covers), **exact-repro** determinism, blended pipeline latency down **77% (= g)**.
   Coverage g is the lever; the residual is the binding constraint. Also surfaced **caching as the
   floor of shift-left** — `.crystal-cache` replays a 710ms model call in µs (THESIS "Memoization is
   the floor"). *Still to do:* the breakeven/amortization point (hits to repay `author`'s one-time
   Opus authoring cost; the re-author frequency that erases the win) and a live hook (rung 1), not a
   batch microbenchmark.
3b. **The residual slice — RESULT LANDED** (`support`, `SUPPORT_FINDINGS.md`). Semantic support
   (paraphrase) — the chore a string tool *can't* cover. det-tool recall on paraphrase = 0/4
   (residual confirmed real); **haiku-whole 1.00 = opus-whole 1.00 @ ~2.6× lower latency** → a cheap
   model covers *easy* semantic support as well as the frontier (shift-left reaches the residual,
   not just the mechanical fraction). Retrieval was a no-op on short sources. **Hard set
   (`support --hard`):** tried to separate cheap from frontier with long buried-needle docs + subtle
   reasoning (quant traps, scope/negation, multi-hop, temporal) — and **couldn't**: haiku 1.00 = opus
   1.00 again (frontier-necessary boundary still unfound; cheap-model reach is larger than expected).
   What DID separate: **retrieval HURT** (0.85 < 1.00) — lexical keyword retrieval from a paraphrased
   claim can't find paraphrased source, so naive RAG can be worse than whole-doc. *Open:* a set that
   actually induces cheap-model errors (longer-than-context docs, denser ambiguity) to find the
   frontier boundary; semantic (not lexical) retrieval.

3. **The LLM-tier slice — FIRST RESULT LANDED** (`payoff`, `PAYOFF_FINDINGS.md`). Shifted a
   mechanical chore Opus→Haiku behind a deterministic gate: **~46% median latency saved**, but
   quality only *mostly* held (0.86 vs 0.93; the deterministic gate leaks in-source semantic errors
   it can't see). The breakeven is demonstrated: deterministic gate = fast + leaky; LLM gate =
   correct + no latency win. *Still to do:* run on *real* (non-synthetic) chores and a real
   agentic-loop baseline; multi-sample latency; an honest LLM-gate latency measurement (not just the
   qualitative claim).
4. **Decomposed shift-left — cheap model + robust tool — FIRST RESULT LANDED** (`decompose`,
   `DECOMPOSE_FINDINGS.md`). Quote/citation verification, three conditions: **det-tool (rg) 1.00 @
   ~0ms > whole-haiku 1.00 @ 612ms > haiku+tool 0.92 @ 745ms.** Lesson, partly *against* the naive
   thesis: when a deterministic tool fully covers the chore, **drop the model** — adding it (whole or
   as driver) is overhead, and the driver role introduced a new failure (its fragment choice dropped
   the distinguishing token → false-present). Decomposition pays only on the residual the tool
   *can't* cover (fuzzy/paraphrase). *Still to do:* a chore with a real uncovered residual
   (paraphrase/semantic-support citation checking), longer/harder inputs (the predicted whole-task
   hallucination didn't surface on the easy set), and the tool-de-biasing angle (weir).
5. **Close the sovereignty gap (the gradient's far end).** Swap the cheap tier to a local small
   model (+ LoRA) on owned hardware; re-measure latency and held quality. The cost gradient is
   `frontier → … → local+LoRA → deterministic hook`; `payoff` currently stops at cloud Haiku. Reuse
   candidate: the local-hybrid work in sibling projects **cupel** and **lexicon** (verify what's
   actually there before assuming). *Proves:* the sovereignty/determinism pitch is real, not
   aspirational. *Done when:* a chore is served from local hardware with the gate intact.

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
- **Tool inventory is a provisioning prerequisite, not a given.** The deterministic/tool tiers only
  exist if the tools are installed (weir's data: a stock host reaches for the 1995 toolbox). A
  crystallized chore that leans on `rg`/`fd` carries a portability dependency — so the harness must
  detect host capability and fall back or declare the dependency (weir's SessionStart manifest +
  apt-install guidance is the reuse). Enriching the inventory grows the tool-coverable fraction; a
  *personal* harness can target the user's specific richer toolbox the mass-market model assumes
  away. Fold capability-detection into any served-hook/decompose rung above.
