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
- ~~The **payoff is unmeasured.**~~ MEASURED (`serve`/`amortize`): ~90,000× on the covered fraction, blended −77%, latency breakeven 43 hits.
- ~~The crystallized artifact is written to disk but **not actually installed/served** as a live hook.~~ SERVED (`hook`): a real PreToolUse hook answering live with 0 model calls + demote-on-drift across process boundaries. The deployed speedup is ~50–110× (process-fork floor ~5.9ms), not the ~90,000× in-process figure; g=0.77 is **in-sample** (g→0.00 on a foreign command stack) — both scoped after the panel (`PANEL_FINDINGS.md`).
- ~~The live loop does not auto-close.~~ CLOSED (`hook-loop`, `HOOK_LOOP_FINDINGS.md`): demote→re-author→gate→swap→re-promote→resume runs autonomously across 24 real processes; the two M-in-W evasions (interleave, terminal-DoS) are fixed. Remaining gap is epistemic: the new class's labels still come from a provided oracle (= A5).
- ~~The **local-model tier** (RTX 3080 + small model + LoRA, per the brief).~~ PROBED (`local-probe`, `A5_PROBE_FINDINGS.md`): the tier is plumbed (`internal/local`, ollama) and measured — but the NAIVE version (qwen2:1.5b, CPU, no GPU) is **not viable**: 0.46 accuracy vs det 1.00 / Haiku 0.76, and ~4× slower (p50 2.5s vs 0.6s). A5 needs the GPU + a stronger/tuned model the brief assumed. Plumbed and measured, not yet paying.
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
1. **Serve the deterministic hook for real, AND close the loop — RESULT LANDED** (`hook` /
   `hook-demo` / `hook-loop`, `HOOK_FINDINGS.md` + `HOOK_LOOP_FINDINGS.md`). `crystal hook` is a real
   Claude Code PreToolUse hook: stdin event → stdout `additionalContext` with the deterministic
   category (0 model calls on the covered fraction), silent defer on the residual, never denies; it
   serves from a swappable `--rules` artifact. `hook-demo` shows DETECTION live (compiled binary, 24
   separate processes, the M-in-W window surviving via the `--state` file, DEMOTES on a container
   burst). **`hook-loop` closes the loop:** demote→re-author→gate→swap-artifact→re-promote→resume,
   autonomously across 24 real processes — the once-drifting class recovers 8/8. The panel-found
   evasions are fixed (cumulative rate gate catches the interleave; `repromote()` fixes terminal
   demotion). *Deployed cost honestly:* ~5.9ms/call process-fork floor → ~50–110× over a model call
   (not the in-process µs). *Host-capability:* pure Go, shells out to nothing → binary-portable, zero
   weir dep (coverage is host-specific though: g=0.77 in-sample, g→0.00 off-stack). *Remaining gap:*
   the re-author's labels for a NEW class come from a provided oracle — no-live-oracle discovery is A5.
   Wiring: `docs/hooks/settings.snippet.json`.
2. **Measure the payoff — FIRST RESULT LANDED** (`serve`, `SERVE_FINDINGS.md`). Served the
   deterministic tier in place of the cheap-model call on the covered fraction of the real Bash
   chore: **~7µs/call vs Haiku p50 640ms (~90,000×) at zero quality cost** (the rule IS the reference
   on what it covers), **exact-repro** determinism, blended pipeline latency down **77% (= g)**.
   Coverage g is the lever; the residual is the binding constraint. Also surfaced **caching as the
   floor of shift-left** — `.crystal-cache` replays a 710ms model call in µs (THESIS "Memoization is
   the floor"). The breakeven is now also DONE (`amortize`, `AMORTIZE_FINDINGS.md`): latency breakeven
   = **43 covered hits** (one ~23.5s Opus authoring call repaid after 43 served commands; corpus has
   17,402 covered → ~405× past). The load-bearing half: **re-authoring more often than once per 43
   hits nets negative** (R=10 → −330%), so *demote-on-drift* (re-author only on sustained drift), not
   detection alone, is what makes it pay. Token breakeven is ~2,944 hits (~70× slower) — the thesis as
   a number: the win is on the latency axis, not the collapsing token axis. *Still to do:* a live
   PreToolUse hook (rung 1), not a batch microbenchmark.
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
5. **Close the sovereignty gap (the gradient's far end) — PROBED, NEGATIVE on this hardware**
   (`local-probe`, `A5_PROBE_FINDINGS.md`). The local tier is plumbed (`internal/local`, an ollama
   HTTP client cached like the cloud one) and measured on the covered fraction: `qwen2:1.5b` on CPU
   scored **0.46 accuracy vs det 1.00 / Haiku 0.76, at p50 2.5s (~4× slower than Haiku)** — not
   viable as a cheap tier, and too weak (0.46) to be the live oracle the `hook-loop` re-author needs.
   *Bonus:* even Haiku matched det only 0.76 inside coverage (compound-command ambiguity), so the
   deterministic tier is the most ACCURATE on its covered fraction, not just the fastest — the thesis
   in one number. *Path to viable A5:* GPU (the brief's RTX 3080, not on this host) + a 7B-class or
   LoRA-tuned model, or a confirm step that ratifies a weak local oracle's label proposals. *Status:*
   plumbed and measured, not yet paying. *Done when:* a chore is served from local hardware at
   accuracy ≥ Haiku and latency ≤ Haiku, with the gate intact.

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
