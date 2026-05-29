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
4. **Trust substrate (the safety reframe — see the re-centering note that follows).** The mechanism is commoditizing.
   What the field is racing *past* is making recursive self-authoring **safe to run
   unattended**. Crystal is the **trust substrate for recursive self-authoring
   loops**: verifier-gated promotion, drift-triggered demotion, tamper-proof
   guardrails, and instrumented per-hop signal loss — the discipline that turns a
   self-improving loop system from a silent-degradation hazard into something you can
   leave running.

   **Topology-general, not just a tier stack.** A vertical stack of tiers is the
   *path* special-case. The same self-authoring composition gives trees (one
   supervisor authoring many parallel sub-harnesses), dev-time cycles (a critic loop
   wrapping a runtime loop — the hybrid-loops dev-time regime), and meshes (co-equal
   loops generating each other's surface). Crystal's primitives are **edge-local and
   node-local** — a verifier gates a promotion *edge*, λ is per-*edge* loss, the
   tamper-proof kernel is a *node* property — so the discipline composes over an
   arbitrary directed graph of loops, not a single ladder. The experiments so far
   exercise only the linear/vertical case; trees, cycles, and meshes are in scope and
   untested.

## Re-centering (2026-05-29): shift-left is the point; trust is the scaffolding

Stage 4 above is *true but secondary*, and it was the assistant's emphasis more than the
project's. The durable core — confirmed by an adversarial prior-art pass (the trust-substrate
framing carries the heaviest prior art: **AI Control**, **reward-tampering**) and by a
"time-traveler from 2032" gut-check (the humble shift-left, especially to a deterministic local
hook, is the part that ages best; the grand trust reframe is the part platforms most likely
internalize) — is the **humble shift-left** itself: crystallize mechanical work down to a
cheaper/deterministic tier behind a gate, and keep it there as patterns drift. That is the value
proposition and the thing to build first (`crystallize`, built; the LLM/local tiers, roadmap). The
trust substrate (verifier gate + drift demotion + the still-unbuilt tamper-proof kernel) is the
*enabler that keeps shift-left from rotting* — necessary, but not the headline. Read the rest of
this doc with that ordering: trust claims are the supporting cast, not the lead.

## Shift-left is intra-task decomposition, not just whole-task downshift (2026-05-29)

The sharper mechanism — and the one the `payoff` leak pointed at. Don't swap the *whole* task to a
cheaper model (that leaks on the hard bits, as measured). Decompose it: a task is a mix of
**mechanical, high-coverage sub-steps** (find this string, parse this, typecheck this — g≈1, a
robust deterministic tool already does it perfectly) and an **irreducible judgment residual** (which
entity is the subject, is this argument sound — 1−g). The cost-optimal architecture hands the
mechanical fraction to the ecosystem's battle-tested wheels (grep, parsers, linters) that the cheap
model merely *drives*, and pays model intelligence only for the residual. An Amdahl's-law view of
LLM cost: your bill is set by the fraction you can't offload to a tool.

- **Worked instances (the user's own):** `publicrecord` verifies a quote with Haiku driving `grep`
  — Haiku fills the smallest gap (what to search, is this a match) while grep does the robust
  matching; `stope` (lamina/poc/dense, not public) builds agents that are "almost formal oracles"
  because their output language is a tool-menu whose outputs are checkable. (Described by the user;
  not yet code-verified inside crystal.)
- **Why it works:** a tool-constrained output has a tiny, *checkable* output space — which is
  exactly crystal's gate from the other end. AutoHarness (PRIOR_ART) is the *synthesize-the-tool*
  version; this is the cheaper *use-the-tool-that-exists* version.
- **The honest limit (our own evidence):** the `payoff` leak (Haiku grabbed the distractor
  `Tom Bradley`) was a **semantic judgment** error a grep tool would not fix. Tools collapse
  *mechanical* difficulty, not *semantic* difficulty. So "cheap model + good CLI tool > frontier
  from scratch" holds **on the tool-coverable fraction**; the optimization problem *is* maximizing
  that fraction and shrinking the judgment residual the cheap tier must cover. Error also migrates
  to tool/arg selection, output interpretation, and multi-step orchestration — shallow,
  checkable-output tool use is the sweet spot.

Ethos: a **cheap lunch, not a free one.** Perfect isn't the goal; scrappy, organic, evolving is —
reuse robust modules, accept the residual leak, let crystallize/demote accrete which decompositions
hold. (See `PRIOR_ART.md` for why per-recurring-chore stateful tiering is an unoccupied niche vs
per-request routers.)

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
3. **A tamper-proof kernel *across a self-authoring / drift loop*** — a verifier the
   supervised tier structurally **cannot rewrite**. *Re-scoped (2026-05-29):* the earlier
   "tamper-proof oversight is the durable, under-served contribution" **overclaimed** — the
   **AI Control** program (untrusted model overseen by a trusted component it can't subvert,
   2312.06942) and the **reward-tampering** lineage (decoupled approval / current-RF) own that
   premise and predate crystal by years. The surviving delta is narrow: the instantiation where
   the overseen thing is *re-authoring its own harness over time* (not a fixed deployed model),
   and structural capability-isolation on that authoring edge. Engineering novelty, not conceptual.

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
the grounding-arc docs). Still unproven is the **tamper-proof re-authoring** half: a guardrail the
lower tier structurally can't corrupt (the DGM antidote), demonstrated live. That's the headline of
the *trust* track — which, per the re-centering above, is secondary to actually shipping shift-left
(serve a crystallized hook, measure the latency/determinism payoff; see `ROADMAP.md`).

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
  `--verbose` per-item inspection — the fifth corrected a *prior crystal finding*. This proves the
  **measurement discipline** is real (a human catching miscounts with `--verbose` — good hygiene);
  it is **not** evidence for the tamper-proof kernel (a structural block on a self-improving tier),
  which is a different mechanism and still unbuilt. Don't let the track record flatter the unbuilt
  guardrail by association.

## The leapfrog (where to get ahead, not behind)

**The "labs solve it first" risk is real.** Capability-isolating a verifier the agent can't reach is
fundamentally sandboxing/privilege-separation — exactly what the platform owners who run the only
self-improving stacks today ship as table stakes. On a 12–24mo horizon, "safe unattended
self-improvement" most likely arrives as an agent-framework feature, not a standalone substrate. So
don't bet the project on out-building labs at oversight. Bet on the two things they structurally
*won't* ship, both of which serve shift-left:

- **The shift-left tool itself, vendor-neutral and local-first** — crystallize *your* mechanical
  work down to a deterministic/cheap tier on *your* hardware. Labs optimize their own stack; they
  don't ship you a sovereign migration of your chores. This is the primary bet.
- **Trust certificates / oversight altimeter** — machine-checkable, *locally-emitted*,
  vendor-neutral attestation per output (which tier served it, verifier coverage, drift-free
  window) and a live per-edge λ readout. The one trust asset a platform won't give you for free
  because it's cross-vendor and local by definition.

Lower-priority trust experiments (only as far as they keep shift-left honest): the un-disableable
verifier (DGM antidote) and adversarial g-hardening (a red-team tier hunts the uncovered residual;
the supervisor auto-authors fresh checks). See `ROADMAP.md` for the build order — shift-left first.

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
