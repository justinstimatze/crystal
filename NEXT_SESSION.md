# NEXT SESSION — crystal handoff (2026-05-29)

Resume cleanly from here. HEAD at the `hook` commit (or later); `go build ./...` + `go test ./...` +
`go vet ./...` all green; all work committed; clean tree.

Prior session added three rungs on top of `triage`: `author` (self-authors the verifier), `serve`
(measures the latency payoff), `amortize` (prices it). **This session closed the last batch→live
gap: `hook` — a real Claude Code PreToolUse hook serving the deterministic tier live (0 model calls
on the covered fraction), with demote-on-drift across real process boundaries** (`HOOK_FINDINGS.md`).

## ⇒ TODO (redo cleanly, do NOT skip): clean from-scratch N=250 --home agreement run
The first N=250 `--home` agreement run ABORTED at 72/250 (qwen3.6:35b blew past the 120s client
timeout — RAM-pressure stall from the ~70% VRAM spill; fixed → 300s default + `OLLAMA_TIMEOUT_S`,
commit `ff3ebee`). A resumed run reuses the FAILED run's 71 cached 35B results. That's valid for
CORRECTNESS (content-hash cache + temp-0 determinism → cached label == fresh label) but a LOOPHOLE
for discipline + LATENCY (resumed run reports mixed-regime cached timings — do NOT cite p50/p99 from
it). **Redo later from a clean slate:** clear this corpus's 35B cache entries, run once under one
consistent policy (300s timeout, or treat timeout-as-abstention — pick one), capture clean latency.
Only then are the N=250 latency numbers citable. Coverage/on-agree from the resume are fine to cite now.

## ⇒ STATUS (2026-06-07 late): local tier DE-RISKED on a real GPU + prior-art swept — next = wire the local agreement oracle

Tree clean; `go build/vet/test` green. Commits this stretch: `f165402` (guard) → `8858b10` (context-
reclaim measured + reflexive vision) → `cb12932` (dispatch) → `abc00d0` (A5 overturned) → `65a7a4f`
(8B frontier + agreement) → `34165a1` (prior-art sweep). **Read `A5_PROBE_FINDINGS.md` (top section)
and `PRIOR_ART.md` (2026-06-07 section).**

**What landed (all measured, all remote — the user's RTX 3080 box orchestrated entirely over HTTP via
`OLLAMA_HOST`; pull/load/unload/`/api/ps` never touched by hand; `internal/local` reads `OLLAMA_HOST`
with zero code change):**
- **A5 negative OVERTURNED** (was a toy-1.5b artifact). Frontier vs det: Haiku 0.78@627ms ·
  qwen3.6:35b **0.76**@3298ms (spills 70% to RAM, `/api/ps`-measured) · qwen3:8b **0.68@219ms** (3×
  faster than Haiku, fully VRAM-resident). A capable local model ties Haiku on accuracy.
- **All-local label oracle found:** 8B & 35B agree on **76%** of commands; on agreement **0.86**
  accurate → trust-on-agreement / abstain-on-disagreement closes the `hook-loop` no-oracle gap with no
  cloud. (Self-caught + retracted a bogus "blended latency 968ms" — escalate-on-disagreement needs
  both models run, so no speedup.)
- **`crystal guard` + `crystal dispatch` built** (constraint-type crystallization + the rule-library
  dispatcher — the scaling architecture; rules-as-data + tested matcher registry). Context-reclaim
  measured small (~1.5%/turn) and tempered. Reflexive vision in `THESIS.md` + `SWEEP_FINDINGS.md`.
- **Prior-art swept (3 parallel passes, converged):** agreement-trust = tri-training (2005) + QBC
  (1992); cascade = FrugalGPT/AutoMix; logprob trigger = UCCI 2026 (raw entropy miscalibrated). The
  defensible seam is the **deterministic gate + demote-on-drift + per-chore + all-local** integration,
  NOT the agreement/cascade. Cite, don't claim.

**⇒ DECIDED NEXT ACTION (ranked by leverage):**
1. **Validate agreement at real N, then wire it into the loop** (highest leverage; the project's
   central open gap). (1a) re-run the two-model agreement on the full `--home` transcript corpus to
   see if 76%-agree / 0.86 holds beyond N=37 [cheap, de-risks]; **report abstention coverage**, not
   just accuracy-on-agreement (prior-art note). (1b) feed local 8B+35B agreement as `hook-loop`'s
   re-author label oracle — framed as tri-training/PoLL feeding **our deterministic gate** (lead
   novelty on the gate+demote, not the agreement). Closes "autonomous self-authoring, fully local."
2. **`crystal sweep`** — auto-detect re-encoded-across-N-projects rules → propose dispatcher library
   entries (the real auto-chunk; no hardware dep).
3. **Proposer-confidence latency trigger** — 8B logprob/entropy → escalate only the uncertain ~24% to
   the 35B. *Prerequisite (UCCI):* calibrate the signal (isotonic/temperature) — raw entropy is
   miscalibrated; this is table-stakes, not optional.
4. **Fold `hook`'s classifier into the dispatcher** (one process, both rule kinds).

*Design fork to decide before more local-model routing:* LoRA-adapter swap (S-LoRA/LoRAX/vLLM
multi-LoRA) instead of separate whole models — near-zero per-task swap on one resident base. *Local
ops cheat-sheet:* models reachable at `http://192.168.4.114:11434`; `qwen3:8b` (resident) + `qwen3.6:35b`
(spills) pulled; `keep_alive:"30m"|0|-1` controls residency; `think:false` REQUIRED for qwen3.x (else
thinking eats the token budget → empty response). Pull remotely via `/api/pull`.

---
## ⇒ STATUS (2026-06-07): vision clarified + crystal swept on its OWN substrate

A conceptual session that produced a sharper statement of what crystal *is*, and a deterministic
sweep applying it reflexively. **Read `docs/SWEEP_FINDINGS.md` and the new "What crystal is, stated
clearly" block at the top of `ROADMAP.md`.**

The vision, crisp: crystal is **auto-chunking + shift-left applied to remembering.** Recall is the
frontier tier (lossy, forgettable, costs standing context every turn); a deterministic artifact (hook
/ `make` target / git config) is the cheap tier (reliable, fires unconditionally, **zero standing
context** — the cost axis that does *not* collapse, unlike per-call tokens). The recursion that names
the target: a memory rule is the first-order "don't make me remember," but applying it is a
second-order act of remembering — *not having to remember to not have to remember* is the collapse to
an environmental constraint. **Promotion trigger:** a rule that recurred *despite already being a
rule*; the deterministic proxy is **re-encoded across N projects**.

**The sweep** (no model calls, no transcript reads — memory-footprint discipline): mined every
`CLAUDE.md` + `~/.claude/projects/*/memory` under `~/Documents`. **156** feedback memories, **758**
rule-candidate lines. Verified cross-project recurrences: `git add -A` ban in **4** projects (beads,
calque, lucida, plancheck); `main`-not-`master` in **3**; secrets-to-files in **3**. `weir` is the
existence proof — a `which`→`command-v` correction already promoted to a *blocking PreToolUse hook*;
the promote-set is new rules of weir's shape, not a new system. **Self-illustrating:** the user wrote
the crystal thesis itself ("auto-fire, stop making me remember") as a standing rule in **≥4** projects.

**Next (decided action, gated on user pick):** build the flagship — a PreToolUse `deny` on
`git add -A|.|--all` — which would retire its 4 memory encodings and convert the sweep's headline
finding from a measurement into a *built* artifact. (Still also open: the A5 path below.)

---
## ⇒ STATUS (2026-05-29 night): A5 PROBED — negative on this hardware; pick the path next session

A5 is now **plumbed and measured** (`local-probe`, `internal/local` ollama client,
`A5_PROBE_FINDINGS.md`). Clean negative: `qwen2:1.5b` on CPU (no GPU on this host) scored **0.46
accuracy vs det 1.00 / Haiku 0.76**, at **p50 2.5s (~4× slower than Haiku)** — not a viable cheap
tier, and too weak (0.46) to be the live oracle the `hook-loop` re-author needs (that gap stays open).
*Bonus finding:* even Haiku matched det only 0.76 inside coverage (compound-command ambiguity) → the
deterministic tier is the most ACCURATE on its covered fraction, not just the fastest (the thesis in
one number). All numbers re-counted from raw `--verbose`.

**Next session — pick the A5 path (the build is gated on a decision, not unblocked work):**
- **(a) GPU + stronger model** — the brief's RTX 3080 (not reachable from this host); a 7B-class model
  on GPU likely closes most of the 0.46→0.76 gap and the latency gap. Needs the hardware online.
- **(b) +LoRA fine-tune on this chore** — the brief's actual bet; a tuned small model could match det
  on covered commands. Most work; highest payoff if it lands.
- **(c) confirm step** — treat a weak local oracle's labels as *proposals* a stronger tier ratifies
  before they train the gate. Cheapest; closes the oracle gap without needing local accuracy ≥ Haiku.
- **(d) call Track A done** — the loop is closed; A5 is honestly scoped as "plumbed, not yet paying."
  Consolidate THESIS/README and stop.

Run `local-probe` ONLY on the small corpus (never `--home`) — local CPU inference holds a ~1GB model
resident; stop it after (`ollama stop qwen2:1.5b`) to free RAM.

---
## (superseded) earlier decided action: A5 — the local-model cheap tier

Two things landed this session and the seam the panel exposed is now CLOSED:

**(a) Adversarial panel** (`docs/PANEL_FINDINGS.md`) overturned three headlines, all re-verified vs raw:
1. "90,000× / 7µs" is in-process only — the live `hook` pays ~5.9ms process-fork → ~50–110× over a
   640ms Haiku call (µs figure scoped to `serve`).
2. "g=0.77" is in-sample — a data-science stack scores **g=0.00** through the real `detClassify`;
   binary-portability ≠ coverage-portability.
3. "the loop closes live" was FALSE at panel time (hook demotes+flags, no code wired it to `author`).

**(b) The seam, wired shut** (`hook-loop`, `docs/HOOK_LOOP_FINDINGS.md`): demote→re-author→gate→
swap-artifact→re-promote→resume runs **autonomously across 24 real processes** — the once-drifting
container class recovers 8/8. The hook serves from a swappable `--rules` artifact; `repromote()` fixes
the terminal-DoS; a cumulative rate gate catches the 2-in-5 interleave. 5 unit tests (both panel
evasions are regression tests). **The loop is mechanically autonomous but epistemically
oracle-dependent:** the re-author's labels for a NEW class still come from a provided reference
(`containerRef`). That epistemic gap IS A5.

**THE BUILD — A5: the local-model cheap tier (the sovereignty end + the live oracle).** Swap the
residual's cheap tier from cloud Haiku to a local small model (+LoRA) on owned hardware (RTX 3080,
per the brief), behind the same gate; re-measure latency + held quality. *Two payoffs:* (1) proves the
sovereignty/determinism pitch is real, not aspirational; (2) a local judge is a candidate **live
oracle** — the missing label source that would let `hook-loop`'s re-author discover a new class's
ground truth without a human/cloud call (closing evasion 3, confidently-wrong-is-invisible, too).
*Scope a probe first* (one local call through the gate, mirroring how `probe` de-risked the cloud
tier). *Reuse candidates:* sibling projects **cupel** / **lexicon** — verify what's actually there
before assuming. *Done when:* a chore is served from local hardware with the gate intact.

## The thesis, current (read `docs/THESIS.md` for the full version)

- **Primary value = humble shift-left**: move mechanical work off the frontier onto cheaper /
  deterministic tiers behind a verifier. NOT token cost (collapsing) — latency, determinism,
  sovereignty, throughput. Trust-substrate framing is *secondary* scaffolding (it was the assistant's
  over-emphasis; user pulled it back).
- **Decompose, don't just downshift**: offload mechanical/high-g sub-steps to robust tools; the LLM
  is a **pattern engine** — use it for fuzzy/semantic judgment, route precise/symbolic work
  (counting, arithmetic, aggregation) to deterministic code. *Don't make a model count.* Marshal the
  whole ensemble (frontier + cheap models, tools, tool-inventory, verification) for collective
  robustness. Don't reach for the LLM just because it's shiny.
- **As LLM authoring → free (the "slam left" correction), verification is the rate-limiter** — which
  puts crystal's verifier-gate at the center. The binding constraint is a *moving frontier* (authoring
  collapsed → verification next); the durable discipline is continuously re-finding it.
- **Novelty = integration, NOT invention.** Each mechanism is published (Blueprint-First = inversion,
  Plan-Caching = crystallize-to-tier, Workflow-Memory = accretion, SSGM = drift-gating, compound-
  engineering = Every). crystal's bet is the *union* + demote-up-a-tier-on-drift + deterministic-
  default + per-recurring-chore-stateful. Don't claim first-to-invert.

## What's built (8 experiments + the v1 slice + self-author + serve/amortize + the live hook)

CLI: `crystal <cmd>` (kong). Experiments measure; `triage` ships; `author` self-authors; `hook` serves live.
- `hook` / `hook-demo` / `hook-loop` — **the live PreToolUse hook AND the closed loop**: a real
  Claude Code hook answering the categorize chore deterministically (0 model calls), serving from a
  swappable `--rules` artifact, demote-on-drift across real process boundaries; `hook-loop` closes
  the loop (demote→re-author→gate→swap→re-promote→resume) autonomously across 24 processes, fixing
  the panel's terminal-DoS + interleave evasions. `HOOK_FINDINGS.md`, `HOOK_LOOP_FINDINGS.md`,
  `PANEL_FINDINGS.md`, `docs/hooks/settings.snippet.json`.
- `amortize` — **prices the artifact** (commit 9daf3b3): latency breakeven **43 covered hits** (one
  ~23.5s Opus author call); **re-authoring more often than once per 43 hits nets negative** (so
  demote-on-drift, not detection, is load-bearing); token breakeven ~2,944 (~70× slower → latency is
  the axis, not tokens). `AMORTIZE_FINDINGS.md`.
- `serve` — **measures the payoff** (commit eda98f5): det tier vs Haiku, ~90,000× on the covered
  fraction at zero quality cost, exact-repro, blended latency −77% (= g). Caching is the floor of
  shift-left (two regimes). `SERVE_FINDINGS.md`.
- `author` — **self-authors the verifier** (commit c6b5946): Opus writes triage's rule table, gated
  (0.93→promote, corrupted 0.01→reject), re-authored on injected drift (8/8 recovered).
  `AUTHOR_FINDINGS.md`.
- `triage` — **v1 SLICE (SHIPPED)**: map-reduce + verifier on a real chore (categorize real Bash
  usage). g=0.77 deterministic, cheap model on the 23% residual, deterministic reduce, **0 frontier
  calls**. `SLICE_FINDINGS.md`.
- `decompose` — tool-coverable chore: det-tool wins outright; model is overhead; the driver fumbles glue.
- `support` / `support --hard` — semantic residual: cheap model = frontier (couldn't find the
  frontier-necessary boundary); retrieval HURT (lexical paraphrase gap).
- `aggregate` — the clean decomposition win: map-reduce (cheap per-item classify + deterministic
  count) beat BOTH whole-task tiers including Opus. "Don't make a model count."
- `payoff` — ~46% latency Opus→Haiku behind a gate, mostly-held quality.
- `ground-hop`/`uncover-hop`/`depth-sweep`/`content-sweep` — g and λ grounding (g=1 byte-exact; λ≈0
  at hop 1; no depth compounding; loss is at hop 1, not depth/format).
- Phase-1: `eval`/`compare`/`drift`/`lattice`/`crystallize`/`measure`/`extract` (the gate, comparators,
  windowed demotion, topology sim, lifecycle, corpus).

## Self-author the verifier — DONE (`author`, `AUTHOR_FINDINGS.md`, commit c6b5946)

The recommended next build LANDED. `crystal author`: Opus authors `triage`'s deterministic rule
table from labeled examples → deterministic gate promotes only at ≥0.90 holdout fidelity vs the
hand-rule reference → windowed-M-in-W drift trigger forces a verified re-author. Live (8,589
holdout): authored **0.93 → PROMOTE**, corrupted negative control **0.01 → REJECT** (gate is
load-bearing), injected container class **DEMOTE@2 → re-author → 8/8 = 1.00 RECOVERED**. Author
fidelity scaled with sample size (200→0.87, 400→0.89, 800→0.93) at a fixed 0.90 bar — every green
came from more data, never a lowered threshold. *Caveat:* fidelity-to-reference (hand-rules), not
ground-truth; still a batch, not a served hook.

## Measure the payoff — FIRST RESULT DONE (`serve`, `SERVE_FINDINGS.md`, commit eda98f5)

`crystal serve` served the deterministic tier in place of the Haiku call on the covered fraction:
**~7µs/call vs Haiku p50 640ms (~90,000×) at zero quality cost** (the rule IS the reference on what
it covers), **exact-repro** determinism, blended pipeline latency down **77% (= g)**. Coverage g is
the lever; the residual is the binding constraint. Also added **caching as the floor of shift-left**
(THESIS "Memoization is the floor"): local memoization (`.crystal-cache`, replays a 710ms call in µs)
AND prompt-cache input-structuring (stable bulk first, volatile last → re-bill only `cache_read`).

## The live PreToolUse hook — DONE (`hook` / `hook-demo`, `HOOK_FINDINGS.md`)

The last batch→live gap is closed. `crystal hook` is a real Claude Code PreToolUse hook (stdin event
→ stdout `additionalContext` with the deterministic category, **0 model calls** on the covered
fraction; silent defer on the residual; never denies; fail-open). `crystal hook-demo` drives the
**compiled binary** over 24 live PreToolUse events — a separate process per command, the M-in-W drift
window surviving only via an on-disk `--state` file — and the tier **DEMOTES live** when a container
burst collapses coverage. Live run: 16 real commands (10 served / 6 deferred, no false demote) then 8
container commands → demote. Contract verified against raw stdout (covered→category, residual→silent,
non-Bash→pass-through, `total` correctly not incremented). *Honest finding:* the live trigger fired
one command into the burst because trailing normal residual had clustered — the coverage trigger sees
coverage, not drift-vs-residual (M-in-W tuning). *Host-capability:* this rule table shells out to
nothing → fully portable, **zero weir dependency**. Wiring snippet: `docs/hooks/settings.snippet.json`.

## THE NEXT BUILD (recommended) — A5: the local-model cheap tier (the sovereignty rung)

Everything cheap so far is cloud Haiku or the deterministic tier. The last open rung is the gradient's
far end:
- **Local small model (+LoRA) on owned hardware** (RTX 3080, per the brief). Swap the residual's
  cheap tier from cloud Haiku to a local model behind the same gate; re-measure latency + held
  quality. *Proves:* the sovereignty/determinism pitch is real, not aspirational. *Reuse candidates:*
  sibling projects **cupel** / **lexicon** (verify what's actually there first — don't assume).
  *Watch:* this is the first rung needing real local-inference plumbing; scope a probe (one local
  call through the gate) before the full slice, mirroring how `probe` de-risked the cloud tier.

## House rules / cautions (earned this session)

- **Verify against raw before any number is a finding.** SEVEN manufactured-confidence catches
  (219 walker miscount; lattice depth; experiment λ=0.90; ground-hop run-1 λ=0; depth-sweep "content
  erodes"; payoff exact-match-gold; support 8-token-truncation default). Always `--verbose` per-item;
  exclude parse-fails from accuracy (never default a class); use `llm.Client.Classify`
  (thinking-disabled) + a real token budget (≥16–24) for tiny verdicts.
- **Never name private projects in public docs**: `publicrecord`, `stope` (lamina/poc/dense) are NOT
  public — anonymize as "a claim-verification agent" / "an auditing agent". `weir` IS citable. Don't
  cite Bostrom / the Oracle-AI strand (TESCREAL; dropped — use Russell's provably-beneficial).
- **`weir` lints the word "which" in prose** as the `which CMD` antipattern and BLOCKS commits —
  reword ("that"/"and that") or `WEIR_SUGGEST_SKIP=1`.
- `go test` runs `go vet` — watch redundant `\n` in `Println` etc.
- LLM calls cached to `.crystal-cache` by content hash (incl. latency); reruns free.

## Memory index

`~/.claude/projects/-home-gas6amus-Documents-crystal/memory/MEMORY.md` — esp.
`shift-left-is-primary-payoff-measured`, `shift-left-is-intra-task-decomposition`,
`crystal-novelty-is-integration-not-invention`, `content-not-format-predicts-channel-loss`,
`lambda-needs-hard-labels-not-a-judge`.
