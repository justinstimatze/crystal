# NEXT SESSION — crystal handoff (2026-05-29)

Resume cleanly from here. HEAD `60163a3` (or later); `go build ./...` + `go test ./...` + `go vet
./...` all green; all work committed; clean tree.

This session added three rungs on top of `triage`: `author` (self-authors the verifier),
`serve` (measures the latency payoff), `amortize` (prices it). The only remaining batch→live gap is
the live PreToolUse hook (see THE NEXT BUILD below).

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

## What's built (8 experiments + the v1 slice + self-author)

CLI: `crystal <cmd>` (kong). Experiments measure; `triage` ships; `author` self-authors.
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

## THE NEXT BUILD (recommended) — the live PreToolUse hook (ROADMAP A1, the last batch→live gap)

The amortization breakeven is DONE (`amortize`). The remaining honest gap is that EVERYTHING is a
batch over a corpus, never a hook in a real loop. Next:
- **Live hook**: emit a promoted artifact (`triage`/`author` rules, or a `crystallize` artifact) as
  an actual Claude Code **PreToolUse hook** that answers a recurring chore in place of a frontier
  call, and demote live on a deliberately introduced drift. *Proves:* the loop closes on live use,
  not just in a benchmark. *Watch:* host capability — the deterministic tier leans on installed tools
  (`weir`'s manifest); detect-and-fallback or declare the dependency.

The far rung (A5): swap the cheap tier to a local small model (+LoRA) on owned hardware — the
sovereignty end of the gradient. Reuse candidates: sibling projects **cupel**/**lexicon** (verify
what's actually there first).

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
