# Live Experiment — first run was INSTRUMENT-INVALID (2026-05-28)

The first live tier experiment (`crystal experiment`) produced three confident numbers —
substitution fidelity opus 0.70 / sonnet 0.70 / **haiku 0.80**, guardrail coverage **g=0.00**,
fuzzy-channel **λ=0.90**. The `--verbose` dump shows **all three are artifacts of the
instrument, not measurements of the world.** Reporting them as λ/g grounding would have been
the same failure as the retracted 219 count and the lattice depth numbers.

## What the raw output showed

**Fidelity — "Haiku beats Opus" is a scoring artifact.** Exact-match against hand-written gold.
Every Opus/Sonnet "miss" was a *more complete* answer penalized:
- `Dr. Raj Patel` vs gold `raj patel` (gold omitted the honorific)
- `regional director for EMEA` vs gold `regional director` (gold truncated the region)
- `leads the data science team` vs gold `data science lead` (gold was a paraphrase)

Haiku "won" only by dropping the honorific and happening to match the arbitrary gold. The metric
rewards terseness-matching-my-gold, not extraction quality. **Cannot rank tiers from this.**

**λ=0.90 — measures the wrong thing.** Haiku's compressed summaries literally asserted *"Correct
extraction, all three fields accurately captured."* Opus-from-summary returned NO on 9/10 — because
the judge prompt asked it to confirm correctness from a **self-assessment with no data attached**,
which it correctly refused to rubber-stamp. The "disagreement" is *"Opus won't trust an
unverifiable vibe,"* not per-hop signal loss. The summary channel carried a verdict, not the
evidence needed to verify it.

**g=0.00 — unmeasured, not zero.** The two "errors" were both scoring artifacts → ~zero real
worker errors → the guardrail had nothing real to catch.

## What a valid instrument needs

1. **Fidelity:** auto-generated unambiguous gold (or semantic/LLM-judge scoring with a rubric),
   not exact-match against hand-paraphrased fields. And a chore where tiers actually differ.
2. **Channel λ:** the summary channel must carry *verifiable content* (the data, compressed), not
   a self-graded verdict; and the full-judge and summary-judge prompts must be calibrated to the
   same bar. Measure disagreement only on a real error population.
3. **g:** needs a genuine error population (inject real drift, or mine real tier disagreements),
   N ≫ 2.
4. **Deeper question:** synthetic extraction is probably the wrong substrate. The λ that matters
   is loss on *real recursive-stack drift signals*, which needs the tamper-proof-kernel / live
   2-tier harness, not a one-shot extraction chore.

## The pattern (stated soberly)

Three of this project's measurement instruments — `measure` (→219), `lattice` (→depth integers),
now `experiment` (→haiku>opus, λ=0.9) — each produced fluent, specific, confident numbers that
dissolved on inspection against raw output. Each was caught only by a verifier checking the claim
against ground truth. That the project's own measurements keep requiring this is not incidental —
it is the thesis (verifier-gated trust) demonstrated on the project itself. The standing lesson:
**no measurement here is a finding until something has checked it against the raw source.**
