# Payoff — does shift-left buy latency at held quality? (2026-05-29)

The first experiment about the *value proposition* rather than the safety discipline. Everything
prior measured whether the gate catches errors (g) and whether the signal survives relay (λ); none
showed shift-left *bought* anything. This one times real API calls.

## Method

Chore: extract `{name, role, org}` from a sentence (known gold), N=14. All tiers use
thinking-disabled completion (a clean mechanical-chore size comparison, not a thinking-budget
confound). Latency is real wall-clock per live call (`llm.Result.LatencyMS`), persisted in cache so
reruns report the originally-measured times.

- **Opus / Sonnet / Haiku** across the cost gradient (Opus = the frontier cost we're avoiding).
- **Haiku behind a deterministic gate** = schema (all fields non-empty) AND substring-grounding (each
  field appears in the source). Accept → serve Haiku; reject → escalate to Opus. The gate is
  deliberately deterministic (no API), because an LLM verifier would re-add an Opus round-trip per
  item and erase the latency win — the breakeven this measures.

## Results (verified against per-item extractions)

**The cost gradient — shift-left isn't binary** (the brief's rule is "cheapest tier that
*realistically succeeds*," not "jump to the bottom"):

```
  opus    accuracy 13/14 = 0.93   median latency 1684 ms   (100% of opus)
  sonnet  accuracy 12/14 = 0.86   median latency 1369 ms   ( 81% of opus)
  haiku   accuracy 12/14 = 0.86   median latency  902 ms   ( 54% of opus)
```

On *this* chore the gradient is effectively **bimodal**, not smooth: Opus at 0.93, or 0.86 at
*either* cheaper tier — and **Sonnet is dominated by Haiku** (identical 0.86, but slower). The
per-item dump shows why: the cheaper tiers fail on the same **distractor** items, the same way.
Item 10 ("…junior engineer Tom Bradley assists principal engineer Anna Cole…") — both Sonnet and
Haiku extract `Tom Bradley` (the distractor); only Opus gets `Anna Cole`. Item 12 ("…his deputy Ines
Roca runs day-to-day operations as COO") — Opus *and* Sonnet both grab `Diego Ramos` (the chair);
nobody gets it. So the quality cliff is **Opus vs everything-cheaper, driven by distractor
robustness** — not a tier-capability ladder you can step down gently. *The useful knee is
chore-dependent;* on an easy chore there may be no beneficial middle stop, which is exactly why you
measure per-chore instead of assuming a gradient.

```
haiku behind the deterministic gate:
  accepted 13/14, escalated 1/14 (the empty output)
  served accuracy 12/14 = 0.86      leaked 1 (in-source distractor the gate can't see)

payoff: gated cascade vs always-opus
  accuracy  0.86  vs  0.93   (delta -0.07)
  latency   902 ms vs 1684 ms → ~46% saved   at the cost of 1 leaked error
```

## What it shows

- **The latency half of the value prop is real and large: ~46% median.** And *conservative* —
  both tiers ran thinking-disabled; with Opus's adaptive thinking on (its normal mode) the gap, and
  the shift-left win, would be larger.
- **The "held quality" half is conditional, not free.** The deterministic gate held against
  *malformed* output (it escalated the one empty Haiku result to Opus) but **leaked an in-source
  semantic error** — Haiku extracted `Tom Bradley`/`junior engineer` (the distractor person), whose
  values are both substrings of the source, so substring-grounding accepted a wrong answer. Served
  quality 0.86 vs Opus 0.93: a 0.07 drop, one wrong answer served silently.
- **The breakeven, demonstrated.** ~46% latency for −0.07 accuracy with a deterministic gate. To
  recover that 0.07 you need a gate that catches the distractor error — i.e. an LLM verifier — which
  adds an Opus call per item and erases the latency win. So shift-left is **"latency at
  *mostly*-held quality, with a measured leak,"** and the real engineering knob is deterministic-gate
  (fast, leaky on semantics) vs LLM-gate (correct, no latency win). This is the FrugalGPT cascade
  tradeoff (acknowledged prior art) measured on crystal's own substrate, with the leak made explicit.

## The sixth catch (two instrument bugs, caught mid-run)

The first run reported opus 0.86 / haiku 0.79 / 3 leaked. Dumping the actual extractions (the
standing rule) showed both numbers were contaminated:

1. **Exact-match gold recurred.** Item 9: gold role `"regional director"`, both tiers returned
   `"regional director for EMEA"` — *more complete, scored wrong by exact-match.* This is the exact
   artifact that invalidated the original `experiment` run, reproduced here. Fixed with bidirectional
   containment matching (`fieldEq`); a wrong-but-disjoint answer still fails.
2. **Empty passed the gate.** Item 12: Haiku returned `""/""/""` and substring-grounding accepted it
   (`strings.Contains(x, "")` is always true). Fixed by adding the non-empty schema leg to the gate;
   the empty now correctly escalates.

After both fixes the honest tally is opus 0.93 / haiku 0.86 / 1 genuine leak. That a payoff
experiment re-introduced an already-retracted artifact is the point, not an embarrassment: the
`--verbose` extraction dump caught it before it became a finding.

## Caveats (bound the claim)

- **Latency is single-sample per item, N=14, with two outliers** (Haiku 8806 ms on item 2, Opus
  19862 ms on item 10 — cold-start / network spikes). Medians are used precisely because they're
  robust to these; do not read the medians as a benchmark.
- **Quality scoring is a containment heuristic, not an LLM judge** — kept out of the headline number
  deliberately (the fallible-judge problem). It removes the terse-gold artifact but isn't semantic
  equivalence.
- **Same 14-item synthetic corpus** as the uncover/depth arc; one tool/chore type. The gradient's
  far end — **local models** (the brief's `frontier → local+LoRA → deterministic hook`) — is
  unmeasured; cloud Sonnet/Haiku are the cheap-tier stand-ins for now. Local-hybrid work in sibling
  projects (cupel and a private sibling) is the likely reuse path when that rung is built (`ROADMAP.md` Track A4).

## Bottom line

Shift-left's latency win is real, large, and easy (~46% here, conservatively). Its "held quality"
rider is exactly as strong as the gate's coverage of the cheap tier's error mode — and a cheap
deterministic gate does **not** cover in-source semantic errors, so quality is held against garbage
but leaks on confident-wrong answers. The value prop is validated for latency and *qualified* for
quality: it's "fast and mostly-right with a deterministic gate, or correct-but-not-faster with an
LLM gate." That tradeoff — not a free lunch — is the honest shift-left.
