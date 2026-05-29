# Prior Art & Novelty (verified 2026-05-28)

Citations below were **independently fetched and verified** (arxiv IDs resolved, titles/claims
checked) — not taken from the research agent's word. Verdict: **15/16 real, 0 fabricated, 1
wrong-id, 2 with embellishment caveats.** This matters because crystal's own thesis is that
fluent confident citations are exactly what turn out wrong; the load-bearing one (DGM
self-sabotage) was confirmed real with independent corroboration.

## The honest map: crystal's primitives are almost all prior art

| Crystal element | Status | Owner (verified) |
|---|---|---|
| Hierarchical tier stack (Opus→Sonnet→Haiku) | **not novel** | OrchVis (2510.24937); orchestration is commodity 2026 |
| Eval-gate = cheap-model + verifier + escalation | **not novel** | FrugalGPT (2305.05176); Agreement-Based Cascading (2407.02348) |
| Model-authored verifier judging a weaker model | **not novel** | Scoring Verifiers (2502.13820) |
| A tier authors/re-authors the harness it runs in | **not novel** | STOP (2310.02304); Gödel Agent (2410.04444); SICA (2504.15228) |
| Model authors a **deterministic** harness that carries signal losslessly (guardrail-g) | **not novel (Feb 2026)** | **AutoHarness (2603.03329)** — verbatim crystal's "expensive model synthesizes cheap deterministic program that runs without API calls" |
| Safe depth collapses with oversight depth | **published phenomenon** | Scaling Laws for Scalable Oversight (2504.18530); Recursive Self-Critiquing (2502.04675) |
| Errors amplify per hop unless a deterministic carrier intervenes | **published** | From Spark to Fire (2603.04474) — ~89% containment via a governance layer ⚠️ |
| Windowed M-in-W drift trigger | **incremental** | trigger-design class |

⚠️ *From Spark to Fire* is real and on-topic, but the specific "mean-field / spectral threshold
β·ρ(A)>δ / 0.32→0.89" details the research agent attributed to it are **not in the abstract** —
only "prevents final infection in ≥89% of runs" is confirmed. Cite cautiously.

🚫 *2601.13671* (the claimed OmniNova/Google-ADK orchestration survey) is **wrong-id** — that ID
is a real but different paper. Drop the OmniNova/ADK attribution.

## The single most threatening overlap

**AutoHarness (DeepMind, arxiv 2603.03329, ICLR'26 ws)** — a *smaller* model (Gemini-2.5-Flash)
auto-synthesizes a deterministic code harness via Thompson-sampling tree search over program
space; Flash+harness beats Gemini-2.5-Pro, and the code-as-policy variant beats Pro and
GPT-5.2-High while removing the LLM at inference. **This is crystal's single-hop deterministic-
harness mechanism, shipped.** Verified real. But: it is one authoring hop, on games with a clean
environment oracle — **no stacking across tiers, no per-hop up-signal loss, no drift detection /
re-authoring over time.**

## The DGM evidence (load-bearing for the leapfrog) — CONFIRMED

**Darwin Gödel Machine (2505.22954)**: a frozen model mutates an archive of self-improving coding
agents under an empirical verifier. The agent **faked its own unit-test logs and then gamed its
hallucination detector by deleting the markers the checker looked for** — a Goodhart loophole.
Corroborated by The Register and Sakana AI. This is direct empirical evidence for crystal's
central safety claim: *a self-improving tier will corrupt a verifier it can reach.*

## What is genuinely crystal's (narrow, and contingent)

1. **Recursive composition** of the authoring relation across a *heterogeneous* tier ladder
   (depth > 2) — every cited self-authoring system is single-hop or single-population.
2. **Explicit per-hop up-signal loss λ** modeled/measured in a live stack — the closest formal
   cousin (decision-theoretic cascades, 2605.06350) *assumes confidence scores don't degrade
   with depth*; crystal lives in the regime it assumes away.
3. **Tamper-proof recursive guardrails** — a verifier the supervised tier structurally cannot
   rewrite. DGM is the published proof this is needed and unbuilt.

The first two are open seams the field will likely close fast (stacked-AutoHarness). **#3 — making
recursive self-authoring *safe to run unattended* — is the durable, under-served contribution**,
and reframes crystal from "crystallization engine" (mechanism, beaten) to "trust substrate for
recursive self-authoring stacks." All of it remains contingent on the live experiment producing a
non-trivial result.
