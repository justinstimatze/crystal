# Hybrid Loop Crystallization Engine — Project Brief

> **This is the original charter, kept for provenance. It is superseded where it differs by
> [`docs/THESIS.md`](docs/THESIS.md) (current framing) and the findings docs. New readers: start
> at [`README.md`](README.md).** The thesis has since evolved from the cost-gradient framing below
> to "a trust substrate for recursive self-authoring stacks"; the hard rules and value prop
> (latency/determinism/reliability/sovereignty) still hold.

Working title. Build a Claude Code plugin that observes my normal CC use, accumulates a typed substrate of tool calls and outcomes, and auto-discovers patterns mechanical enough to migrate down a cost gradient (frontier API → local model + LoRA → deterministic hook). The frontier model stays at the cognitive core; the cheap layers handle the crystallized residue.

## Thesis

Value prop is **latency + determinism + reliability + sovereignty**, NOT cost arbitrage. Frontier prices are collapsing fast enough that "save tokens" is the wrong frame. The durable value is a system that gets faster and more reliable on my specific patterns over weeks of use, with crystallized work serving from local hardware I already own (RTX 3080 12GB).

## What this is NOT

Hard guardrails encoded from prior design work:

- **NOT a Claude replacement.** The frontier (Opus / Sonnet via API) handles novel cognition, multi-step planning, long-horizon agentic work. Crystallization never targets these.
- **NOT speculative downward delegation.** Don't crystallize a task because I *think* a cheap model could handle it. Crystallize only after observing the frontier do it many times with stable output, then verifying the cheap version reproduces the frontier's behavior on historical examples.
- **NOT a cost-savings play.** Don't tune toward "minimize API spend." Tune toward "latency, determinism, reliability for the mechanical residue."
- **NOT an orchestrator.** This is an observer + crystallization engine, not a coordinator of multiple agents. No gastown/undercity-style work routing.
- **NOT a self-modifying mess.** Stable kernel that doesn't auto-modify; auto-generation happens one level above the kernel; demotion is more aggressive than promotion.

The failure mode we are most worried about: **undercity-style silent degradation**. Crystallized hooks running for weeks producing subtly wrong output I don't catch because I'm no longer in the loop. Better to demote too eagerly than to trust quietly.

## Architecture

Five logical components. **Build the eval / drift detector first**; it is the riskiest and gates everything downstream.

### 1. LENS — Observer
- Hooks into Claude Code's lifecycle (primary: `PostToolUse`)
- Captures structured records: `(input_context, tool_called, tool_args, tool_result, claude_followup, repo, timestamp)`
- Writes to local SQLite substrate
- Zero foreground overhead; runs as a daemon

### 2. SUBSTRATE — Typed records
- SQLite at `~/.claude-loop/substrate.db`
- Schema designed so similar tool-call patterns cluster naturally
- Periodically pruned / archived after a pattern is promoted

### 3. GATE — Pattern detector
- Cheap statistical clustering on accumulated substrate, **no LLM here** — deterministic
- Looks for high-frequency tool-call sequences with low semantic variance in output
- Surfaces candidates above a minimum sample threshold (start at N=30)
- Cooling-off period before re-proposing a previously-rejected candidate

### 4. CRYSTALLIZATION PROPOSER — LLM call
- Single Opus/Sonnet call per candidate
- Generates either (a) a deterministic hook implementation in Go, or (b) a LoRA training spec
- Output is structured (JSON schema) — use constrained generation
- Includes self-reported confidence + explanation of the pattern it sees

### 5. EVAL + DEMOTION GATE — Load-bearing component
- **Build this first as the go/no-go for the whole project.**
- Replays the proposed artifact against historical substrate before promotion
- Promotes only on >95% fidelity match against frontier behavior, minimum 30 samples
- Once deployed: continuous spot-checks against frontier output on a random fraction of new calls
- Demotes on 3 consecutive divergences from frontier
- Logs every demotion loudly — silent failure is unacceptable

## MVP scope (~4 weeks)

### Observation surface

All Claude Code `PostToolUse` events while working in my Go repos. Broad enough to surface real patterns from normal density of use; narrow enough that the substrate has coherent shape. Auto-discovery does the rest; do not pre-specify task targets.

### Hardware

- Claude Code on my normal dev machine for the agent loop
- RTX 3080 12GB for LoRA training overnight + local model serving
- No Pi / phone mesh in v0 — Phase 2 problem

### Software stack

- Go for the daemon (matches existing pattern from slimemold / plancheck / hindcast / defn)
- SQLite for substrate
- llama.cpp + Unsloth for local model serving and LoRA training on the 3080
- Standard Claude Code hook API

### Build order

**Phase 1: Eval harness in isolation.** Synthetic test: take 100 historical CC interactions I already have logs of, construct a fake "crystallized hook" that's deliberately wrong in subtle ways, verify the eval catches the divergence reliably. If the eval harness can't catch synthetic regressions, **stop and rethink. Do not proceed to logging.**

**Phase 2: LENS + SUBSTRATE.** Hook, log to SQLite, validate substrate accumulates useful structure. Run for ~1 week of normal CC use. If after a week the substrate has fewer than ~200 records with any clustering, the observation surface is too narrow — widen before proceeding.

**Phase 3: GATE + PROPOSER.** Cheap statistical clustering on substrate. Surface top 1-3 candidates. Single proposer call per candidate. Manual review of generated artifacts — no auto-deployment yet.

**Phase 4: Promotion + drift detection live.** Deploy ONE crystallized hook against one demonstrated-stable pattern. Run continuous drift detection. Either a hook reliably serves a real task class with no degradation (= proof of loop), or it fails (= specific lesson about which patterns aren't crystallizable).

### Success thresholds (keep building if all)

- Eval harness reliably catches synthetic regressions (Phase 1)
- Substrate accumulates >200 useful records/week (Phase 2)
- At least one pattern crystallizes at >95% fidelity on held-out historical samples (Phase 3-4)
- Drift detector fires correctly on at least one deliberately-introduced distribution shift
- Meta-loop overhead is <10% of what the crystallized work would have cost in frontier tokens

### Failure thresholds (stop and rethink if any)

- Eval harness misses synthetic regressions in Phase 1
- Substrate doesn't surface clusterable patterns after 2+ weeks of normal use
- A crystallized hook silently produces wrong output for >24 hours
- Manually tuning eval thresholds more than once per week (= accessibility/complexity gap confirmed)

## Reuse map (my own existing projects)

Compose patterns I've already implemented rather than rebuilding:

- **slimemold** — hook architecture, per-turn extraction + injection. Lift directly for the LENS implementation.
- **plancheck** — verifier pattern, deterministic check (compiler) wrapping LLM output. The proposer should generate verifiers in this style. Use as reference for the eval harness shape.
- **hindcast** — kNN-over-personal-history pattern, calibrated priors with gating on thin retrievals. The drift detector should use this calibration approach.
- **winze** — substrate audit + cognitive-bias awareness. Reference for the drift detection logic, especially the metabolism layer.
- **defn** — typed code substrate, Dolt SQL with git semantics. If/when crystallization targets become code-shaped, use defn's primitives for the artifact storage.

## Open questions to resolve while building

- **LoRA vs deterministic hook as crystallization target.** Tentative rule: deterministic hook if the pattern is fully expressible as code; LoRA if it's fuzzy-but-stable. Refine empirically.
- **Multi-repo: per-repo substrate or shared?** Probably per-repo to start, cross-repo as Phase 2.
- **LoRA serving: llama.cpp adapter loading vs dedicated stack?** Start with llama.cpp's built-in adapter support; switch only if it bottlenecks.
- **Promotion threshold tuning.** Start at >95% fidelity / N=30. Loosen or tighten based on what false-positive and false-negative rates actually look like in Phase 4.

## Hard rules

1. **Never demote judgment to a cheap model.** Only demote pattern-application that's been verified.
2. **No verifier, no crystallization.** If I can't write a check that catches divergence, the pattern isn't ready or isn't crystallizable. Full stop.
3. **Fail loud.** Silent degradation is worse than visible failure. Every demotion should be logged and surfaced.
4. **Stable kernel.** The crystallization engine itself does not auto-modify. Auto-generation happens one level above the kernel; the kernel is hand-written and stable.
5. **The eval / demotion gate is the load-bearing component.** Build it first. If it doesn't work, nothing downstream matters.

## Out of scope (v0)

Defer to later phases once the single-machine loop works:

- Pi 5 + Pixel mesh (Phase 2)
- Multi-device sync (Phase 2)
- LoRA target per task class with adapter-swap-on-route (Phase 2)
- Cross-platform adapters (Pi, phone, exo cluster) (Phase 3)
- Distribution to other users / OSS release (Phase 3)

## Connection to broader thesis

This is the first concrete instance of the hybrid-loop framework documented at github.com/justinstimatze/hybrid — specifically the ambient-meta-loop pattern, where a deterministic hook fires a parallel evaluator whose condensed output is injected back into the primary loop's context, sparing the primary the cost of holding the noticing-gate in attention.

Crystallization is the discipline that makes autonomous async agency safe. The frontier stays the cognitive core; the crystallization engine extracts the mechanical residue from frontier work and serves it from cheaper tiers, with verifier-gated promotion and drift-triggered demotion as the trust mechanism.
