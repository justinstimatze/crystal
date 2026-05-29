# NEXT SESSION — crystal handoff (2026-05-29)

Resume cleanly from here. HEAD `365c3b0` (or later); `go build ./...` + `go test ./...` green; all
work committed; clean tree.

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

## What's built (8 experiments + the v1 slice)

CLI: `crystal <cmd>` (kong). Experiments measure; `triage` ships.
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

## THE NEXT BUILD (recommended) — self-author the verifier

`triage`'s rules are hand-written. The actual crystal mechanism: **the expensive tier authors the
deterministic classifier, gated, re-authored on drift.** Concretely:
1. **Author**: Opus reads a sample of real Bash commands + the category set, emits a rule table
   (pattern → category) as JSON.
2. **Gate**: apply authored rules to a holdout; promote only if accuracy ≥ threshold vs labels
   (hand-rules in `detClassify`/`segClassify` in `cmd/triage.go`, or Haiku-on-residual, as the
   reference). No-verifier-no-crystallization, enforced.
3. **Serve + drift**: run the authored rules; inject a new command class; show the windowed-M-in-W
   demotion fire → Opus re-authors. Head-to-head: Opus-authored rules vs hand-authored, hard labels.
   The key measurement (per the thesis): does the gate reliably catch *bad* authored rules?

Alternative rung (ROADMAP A1): serve a crystallized artifact / `triage`'s rules as a *live*
PreToolUse hook in place of a frontier call; measure real latency in the loop.

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
