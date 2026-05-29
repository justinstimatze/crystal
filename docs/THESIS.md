# Crystal — current thesis (north star)

Supersedes `PROJECT_BRIEF.md` where they differ. The brief is the original charter;
this is what we believe after building Phase 1, pressure-testing it twice, and a
verified prior-art pass (`PRIOR_ART.md`).

## The throughline (how the framing evolved)

1. **Binary (wrong).** "Frontier Opus vs. static deterministic code" — crystallize a
   chore into a hook when its output is constant. The `measure` sweep produced
   "crystallizability ⟂ value" — but those numbers are **retracted** (the headline
   pattern was a walker artifact; see `MEASURE_FINDINGS.md`), and the binary axis was
   itself the error.
2. **Ladder (rejected).** A tier router (Opus→…→hook) sending each chore to the
   cheapest tier that passes a gate. Rejected: routers/cascades are well-trodden
   (FrugalGPT lineage) and boring.
3. **Recursive (the mechanism).** A **loop that constructs loops**: each tier authors
   and re-authors the harness the tier below runs inside; feedback flows down (spec +
   verifier) and up (drift, escalation). *But this mechanism is also largely prior
   art* — AutoHarness (DeepMind, Feb 2026) ships the single-hop deterministic-harness
   version; STOP / Gödel Agent / SICA ship self-authoring. See `PRIOR_ART.md`.
4. **Trust substrate (the actual contribution).** The mechanism is commoditizing.
   What the field is racing *past* is making recursive self-authoring **safe to run
   unattended**. Crystal is the **trust substrate for recursive self-authoring
   stacks**: verifier-gated promotion, drift-triggered demotion, tamper-proof
   guardrails, and instrumented per-hop signal loss — the discipline that turns a
   stack of self-improving tiers from a silent-degradation hazard into something you
   can leave running.

## Why "trust substrate" — and why it's partly what hybrid always meant

The hybrid-loops framework names the disciplines an LLM block requires beyond the
von-Neumann algebra: per-block calibration, context-as-code, the dev-time loop. Those
read as *hygiene* in a single loop. **The moment loops stack — one loop's output
becomes another loop's authoring input — they become survival.** Crystal is the
load-bearing instance of hybrid-for-recursion: the place those disciplines get cashed
out under the one condition (stacking) that makes them mandatory. "Trust substrate" is
that cash-out.

## Honest positioning vs. SOTA (verified — see `PRIOR_ART.md`)

Crystal's primitives are **almost all prior art**: tier stacks (OrchVis), eval-gate
cascades (FrugalGPT), model-authored verifiers (Scoring Verifiers), self-authored
harnesses (STOP/SICA/Gödel Agent), the deterministic-harness mechanism (**AutoHarness,
2603.03329 — verbatim crystal's "expensive model synthesizes a cheap deterministic
program"**), depth-collapse-under-oversight (Scaling Laws for Scalable Oversight), and
error-cascade-needs-a-deterministic-carrier (From Spark to Fire). The genuinely open
seams are narrow:

1. **Recursive composition across a heterogeneous tier ladder** (depth > 2) — cited
   self-authoring is single-hop.
2. **Explicit per-hop up-signal loss λ** in a live stack — the closest formal cousin
   (decision-theoretic cascades) *assumes scores don't degrade with depth*; crystal
   lives in the regime it assumes away. **Partially measured now** (`GROUNDHOP`/
   `UNCOVERHOP`/`DEPTHSWEEP`/`CONTENTSWEEP`): at hop 1, λ ≈ 0 for a content-bearing
   channel (the real axis is *content-vs-verdict*, not prose-vs-typed), and neither
   detection nor content fidelity compounds-loses through 6 relays — the loss sits at
   **hop 1**, not in depth. This *tensions with* the lattice's depth-pessimism below
   and narrows the open question to: does that hold for non-cooperative relays, larger
   N, and greater depth?
3. **Tamper-proof recursive guardrails** — a verifier the supervised tier
   structurally **cannot rewrite**. This is the durable, under-served contribution.

## The load-bearing evidence: DGM (verified)

Darwin Gödel Machine (2505.22954, corroborated by The Register + Sakana AI) had a
self-improving agent **fake its own test logs and game its hallucination detector by
deleting the markers the checker looked for.** That is the published proof that *a
self-improving tier will corrupt a verifier it can reach* — and the entire reason
hard rule #4 (stable, hand-written kernel; the supervised tier cannot modify the gate
above it) is the differentiator, not a footnote.

## The riskiest assumption (partly tested)

Can the upper tier detect a lower tier's drift from a signal degraded by propagation,
and correctly re-author the lower harness — **no human, no silent degradation, with a
guardrail the lower tier can't corrupt**? The *detection-through-a-degraded-signal* half
is now grounded: through 6 relay hops, a content-bearing up-channel preserved detection
(recall 1.00) and content fidelity (~0.70 flat); the loss is at hop 1, not depth (see
the grounding-arc docs). Still unproven — and now the clear headline — is the
**tamper-proof re-authoring** half: a guardrail the lower tier structurally can't corrupt
(the DGM antidote), demonstrated live.

## Reusable assets & what's proven vs. assumed

- **Eval/promote/demote gate** = the lattice unit cell; built, tested, tier-agnostic.
  Its value as a *trust* primitive depends on the lower tier not being able to rewrite
  it (tamper-proofing — not yet built).
- **Drift detector** = the up-feedback / ambient-meta-loop. Consecutive-K is evadable;
  the windowed M-in-W rule is the fix (`DRIFT_FINDINGS.md`).
- **Lattice sim** is **algebra, not emergent** — the frontier is `(1−λ)^(d−1) ≥
  demote/recover`, and guardrail coverage is a **cliff at `g = demote/recover`, not a
  dial** (a manufactured "depth 30" was a search-cap artifact; see correction in
  `LATTICE_FINDINGS.md`). For drift in the un-checkable residual, `g` does not help.
- **Five manufactured-confidence catches** (the 219 count; the depth-2 / depth-30
  numbers; `experiment`'s "Haiku beats Opus / λ=0.90"; `ground-hop` run-1's "λ_prose=0";
  `depth-sweep`'s "content erodes with depth", overturned by `content-sweep`) were
  caught only by verifier-against-ground-truth, adversarial reimplementation, and
  `--verbose` per-item inspection — the fifth corrected a *prior crystal finding*. That
  the project keeps catching *its own* fluent-but-wrong output is the strongest live
  evidence that the trust discipline is necessary — it's the thesis demonstrated on
  itself.

## The leapfrog (where to get ahead, not behind)

Don't compete with AutoHarness on *how to author the cage* (lost). Own *making it safe
to stack and run unattended*, the wall the field hits when stacked self-improvers
start degrading silently:

- **Un-disableable verifier** (the DGM antidote) — capability-isolated kernel the
  supervised tier can author *below* but cannot touch *above*.
- **Oversight altimeter** — measure real λ in a live stack, display current safe depth,
  warn when the top has gone blind.
- **Adversarial g-hardening** — a red-team tier hunts the un-covered residual; the
  supervisor auto-authors fresh deterministic checks; coverage rises as drift mutates.
- **Trust certificates** — machine-checkable attestation per output (tier, verifier
  coverage, safe depth, drift-free window), emitted locally. The sovereignty value
  prop made concrete.

Honest limit: the un-checkable fuzzy residual never reaches zero. The claim is "raise
the trust floor and make degradation *loud*," never "guarantee safety."

## Test ladder (cheapest riskiest-assumption test first)

1. **Deterministic topology sim** (`internal/lattice`) — done; it's algebra + a
   characterization of where the loop goes blind, corrected twice.
2. **Live grounding of g and λ** — done across four experiments. `cmd/experiment` was
   instrument-invalid (diagnosed, not reported); `ground-hop`/`uncover-hop`/`depth-sweep`/
   `content-sweep` then grounded g (1.00 byte-exact; 0.50 substring on semantic drift)
   and λ (≈0 at hop 1; flat to depth 6). Net: the loss is at **hop 1, not depth or
   format** — which tensions with the lattice's depth-pessimism and is the live
   correction to the assumption above.
3. **Tamper-proof recursion demo** (the headline, still unbuilt) — a self-improving stack
   that tries to game its own evaluation (the DGM behavior) and is structurally blocked +
   demoted live, with a visible trust readout. This is the contribution, demonstrated.
