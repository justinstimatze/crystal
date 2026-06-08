# Roadmap

Priority order reflects the project's actual interest: **the humble shift-left is the point.** The
trust substrate is the safety scaffolding that keeps shift-left from rotting — necessary, but
secondary, and heavily prior-arted ([`PRIOR_ART.md`](PRIOR_ART.md)). Milestones are "done when
*measured*," not "done when it exists."

## What crystal is, stated clearly (the vision)

Crystal is **auto-chunking + shift-left applied to remembering.** An expert chunks a recurring
N-step procedure into one named unit so working memory holds one token, not N; Claude doesn't do that
across sessions — it re-derives the dance every time. Crystal detects the recurring pattern, binds it
into one deterministic named unit (a hook, a `make` target, a git config), and serves it from the
*environment* instead of from attention.

Seen this way, **recall is the frontier tier and a deterministic artifact is the cheap tier.**
Carrying a standing rule in memory and re-applying it every turn is expensive, lossy, and
forgettable; the artifact is reliable, fires unconditionally, and — the cost axis that does *not*
collapse — **costs zero standing context on every future turn** (the rule leaves the prompt). The
per-call token figure collapses (see rung 2); context-budget-reclaimed-forever does not.

The recursion that names the target: a memory rule is the first-order "don't make me remember"; it
still fails because *applying* it is a second-order act of remembering. The fix collapses the
recursion — move the constraint into the environment where the wrong path is unavailable. *Not having
to remember to not have to remember.*

**Promotion trigger** (sharper than "recurs N times"): a rule that **recurred despite already being a
rule.** The deterministic, verifiable proxy the substrate sweep found is **re-encoded across N
projects** — the same rule independently re-written in N memories means recall failed to generalize
([`SWEEP_FINDINGS.md`](SWEEP_FINDINGS.md)). **Reflexive target:** crystal's first customer is its own
standing instructions — `weir` already proves the shape (a recurring correction promoted to a
blocking PreToolUse hook).

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
- ~~The live loop does not auto-close.~~ CLOSED (`hook-loop`, `HOOK_LOOP_FINDINGS.md`): demote→re-author→gate→swap→re-promote→resume runs autonomously across 24 real processes; the two M-in-W evasions (interleave, terminal-DoS) are fixed. ~~Remaining gap is epistemic: the new class's labels still come from a provided oracle (= A5).~~ CLOSED too (see next bullet — `--oracle local` discovers the labels locally).
- ~~The **local-model tier** (RTX 3080 + small model + LoRA, per the brief).~~ VALIDATED + WIRED INTO THE LOOP (`local-probe`, `hook-loop --oracle local[-confirm]`, `A5_PROBE_FINDINGS.md`). On the user's RTX 3080 over the LAN (zero code change): `qwen3.6:35b` ties Haiku (0.76 vs 0.78); `qwen3:8b` is ~2.5× faster than Haiku (p50 225ms, VRAM-resident) but 0.69. **Two-model 8B+35B agreement validated at N=250** (replicated across two draws): coverage **0.74–0.80**, accuracy-on-agree **0.85–0.87** > both solo, errors concentrated in the abstained set — an **all-local label oracle**, now **wired into `hook-loop`'s re-author** (`--oracle local`): the loop closes with **no cloud, no human**. A **cascade** (`--oracle local-confirm`, `--confirm-model`) pays cloud only on the abstained slice; the ladder on the container drift class: local 4/8 truth (0 cloud) → Haiku-confirm 6/8 (5 calls) → Opus-confirm+tiebreak **8/8 PROMOTE**. The session's sharpest finding: the gate can REJECT a truth-perfect table when the oracle (agreement=0.85≠1.0) is wrong on a label — producer-verifier gating anti-correlates with truth at the margin; resolved by a gate-time confirm tiebreak (adjudicate conflicts via the strongest tier). Prior art swept (`PRIOR_ART.md`): the seam is the deterministic gate + demote-on-drift, not the agreement/cascade. Open: LoRA; the strong tier's VRAM-fit latency; option-2 (gate at larger N to harden the finding past N=8).
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
5. **Close the sovereignty gap (the gradient's far end) — VALIDATED (N=250) + WIRED INTO THE LOOP +
   a cloud-confirm cascade; the all-local oracle now feeds `hook-loop`'s re-author with no cloud/human**
   (`local-probe`, `hook-loop --oracle local[-confirm]`, `A5_PROBE_FINDINGS.md`). The 2026-05-29
   negative (`qwen2:1.5b`, CPU, 0.46) was a **toy-model artifact** — overturned 2026-06-07. Measured
   on the user's RTX 3080 over the LAN (`OLLAMA_HOST`, **zero code change** — `internal/local` already
   reads it; the GPU box is orchestrated entirely over HTTP: pull/load/unload/`/api/ps`, never touched
   by hand). The frontier, all verified against raw:

   | tier | acc vs det | p50 | p99 | VRAM |
   |---|---|---|---|---|
   | Haiku (cloud) | 0.78 | 627ms | 1102ms | — |
   | qwen3.6:35b (local, spills 70% to RAM) | 0.76 | 3298ms | 22523ms | 30% |
   | qwen3:8b (local, fully resident) | 0.68 | **219ms** | 424ms | 100% ✓ |

   A capable local model **ties Haiku on accuracy** (0.76); the 8B is **3× faster than Haiku** but too
   weak solo. The 35B's latency is a model-VRAM-fit issue (24GB on a 10GB card → CPU offload, measured
   via `/api/ps`), not GPU absence. **Two-model agreement (N=37):** 8B and 35B agree on **76%** of
   commands and on agreement are **0.86** accurate — an **all-local TRUST signal** (trust on agreement,
   abstain/escalate on disagreement) that closes the `hook-loop` no-oracle gap with no cloud. *Caught
   + retracted* an invalid "blended-latency 968ms" (escalate-on-disagreement needs both models run →
   no speedup). *Prior art (see `PRIOR_ART.md` 2026-06-07):* agreement-trust = tri-training/QBC;
   cascade = FrugalGPT/AutoMix; logprob trigger = UCCI (raw entropy miscalibrated → calibration is
   table-stakes); orchestration = commodity. *Steal:* LoRA-adapter swap (S-LoRA/LoRAX) over whole-GGUF
   swap; vLLM Sleep Mode for the warm ratifier. *Status:* accuracy de-risked, agreement-oracle found,
   not yet wired into the loop. *Done when:* a chore is served from local hardware at accuracy ≥ Haiku
   and latency ≤ Haiku, with the gate intact, AND the re-author draws labels from local agreement.

6. **Crystallize crystal's own standing rules — SWEPT + FLAGSHIP BUILT**
   (`SWEEP_FINDINGS.md`, `cmd/guard.go`). The reflexive application: instead of categorizing Bash usage, mine the
   user's own `CLAUDE.md` + memory across all `~/Documents` repos for standing rules and partition
   them by whether a deterministic oracle exists (mechanizable → promotable to a hook/config) vs
   semantic (recall-only, honestly un-promotable). Deterministic sweep (no model calls, no transcript
   reads): **156** feedback memories + **758** `CLAUDE.md` rule lines; the sharp signal is
   **re-encoded-across-projects** (recall failed to chunk) — `git add -A` ban re-written in **4**
   projects, `main`-not-`master` in **3**, secrets-to-files in **3**. `weir` is the existence proof
   (a `which`→`command-v` correction already promoted to a blocking PreToolUse hook). The promote-set
   is *new rules of weir's shape*, not a new system. *Self-illustrating:* the user wrote the crystal
   thesis itself ("auto-fire, stop making me remember") as a standing rule in **≥4** projects — a
   rule that had to be re-remembered per project. **Flagship BUILT** (`crystal guard`, `cmd/guard.go`):
   a real PreToolUse hook that DENIES `git add -A | . | --all` with a stage-explicit-paths reason,
   verified end-to-end over the real stdin contract (deny / silent-allow / non-Bash pass-through /
   `CRYSTAL_GUARD_SKIP=1` override). It ships as a **self-monitoring sub-hybrid-loop**, not a dead
   rule: a constraint produces no answers to verify, so its drift signal is **override frequency** —
   the `--state` file counts denied-vs-bypassed and a sustained bypass rate flags `NeedsRevision`
   (the constraint analog of the categorizer hook's coverage-collapse demote). Wiring:
   `docs/hooks/guard.settings.snippet.json`. *Still recall-only (the next promotes):* `main`-default
   via `git config`, Co-Authored-By trailer, `gh repo create` private-default, secrets-to-files
   linter, end-of-turn `/schedule`-offer Stop-linter.
   **Dispatcher BUILT — first cut** (`crystal dispatch`, `cmd/dispatch.go`). The scaling architecture:
   ONE PreToolUse hook process loads a rule LIBRARY and evaluates every rule in-process (vs one
   process-fork per rule — ~5.9ms × N, seconds at N=1000). The honest data/code split: a rule is
   **data** (id, matcher-name, reason, enabled, per-rule self-monitoring state) so the library scales
   to thousands and is per-user/shareable; each rule references a **named matcher from a small tested
   registry** (`git_add_all`, …) rather than an arbitrary regex (a regex-per-rule library would
   reintroduce guard's false-deny risk). Subsumes `guard` (its rule is the default library's first
   entry). Verified end-to-end: default-library deny, a custom library file with the rule disabled
   allows (data drives behavior), per-rule state keyed by id, fail-open on disabled/unknown-matcher/
   broken-library. Wiring: `docs/hooks/dispatch.settings.snippet.json`, `docs/hooks/library.example.json`.
   This answers the registry question (the library file IS the registry) and the **public/personal
   split** (engine + matcher vocabulary ship publicly; each user grows their own library). *Still to
   do:* classifier-type rules in the library (today it's constraint/deny rules — the categorizer
   `hook` is still its own process); auto-population of the library from the sweep (rung 3 — `crystal
   sweep`); the remaining promote-set rules added as library entries + matchers, not subcommands.

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
