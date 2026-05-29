# Decompose (A4) — cheap model + tool vs whole-task model (2026-05-29)

The first experiment on the *decomposition* refinement: does a cheap model driving a robust CLI tool
beat shifting the whole chore to the cheap model? Chore: **quote/citation verification** (is this
claimed quote actually present in the source?) — a verification-shaped task, and exactly the
"leave citations in, have a cheap agent check them" pattern. Tool: `rg -F -i` (the modern tool
[weir] prefers over grep). Hard labels by construction across categories: verbatim, case-variant,
absent, and *fabricated-plausible* (a flipped digit, or the real Tolstoy line where the source
twisted it).

## Results (N=13, 6 present / 7 not; verified against per-item rows)

```
condition     accuracy   latency        notes
whole-haiku     1.00      612 ms median   Haiku judges from source+quote
det-tool        1.00      ~0 ms           rg -F -i on the full quote, no model
haiku+tool      0.92      745 ms median   Haiku picks a fragment, rg decides — 1 false-present
```

## What it shows (partly against the naive thesis)

1. **For a chore a deterministic tool fully covers, the tool alone wins — outright.** `rg` on the
   full quote got 1.00 at ~0ms with no model. Every "present" item is a verbatim/case-variant
   substring; every "absent"/"fabricated" is not — so exact case-insensitive matching is complete.
2. **The whole-task model matched accuracy but paid ~600ms for nothing.** And the *predicted*
   failure — hallucinating "present" on a fabricated-plausible quote (the real Tolstoy line, a
   flipped digit) — **did not surface**: Haiku correctly called all 7 not-present items absent. So
   on this short/easy set the model is accurate-but-pointless, not dangerous. (Caveat below.)
3. **The model-as-driver was the *worst* condition** — slowest (745ms) and wrong once. Item 5
   (fabricated "43% reduction"): Haiku's chosen fragment was `"reduction in symptoms"`, **dropping
   the "43%"** — the exact token that makes it a fabrication — so `rg` matched the source's "34%
   reduction in symptoms" and returned a false **present**. The tool didn't fail; the model's
   *input selection* (the glue) failed. This is the "error migrates to tool/arg selection" caveat,
   demonstrated live.

## The refined thesis

Decomposition is **not** free and **not** always a win. It pays only when the tool *can't* cover the
chore alone (the fuzzy/paraphrase residual). When the tool *can* cover it:

- **use the tool alone — no model.** That's crystal's v0 (`crystallize` a recurring deterministic
  command). This experiment is direct evidence for the deterministic-default inversion: the model is
  overhead here, not help.
- adding a model as a *driver* can be actively worse, because choosing the tool's input is itself a
  fumble-prone judgment (it dropped the distinguishing token).

Applied to the user's framing — *"leave citations in, have a cheap agent check them" rather than
making the big model get them right inline* — the sharper version is: for **verbatim** citation
checking, you may not need the cheap *agent* at all; a deterministic `rg` check verifies it
perfectly and faster. The cheap agent earns its keep only on **semantic** citation support
("does this source actually back this claim?") — the residual a string tool can't see. Separate
generation from verification (producer-verifier asymmetry, real); then push the verification to the
*cheapest tier that covers it*, which for verbatim is deterministic, not a model.

## Caveats (bound the claim)

- **The dangerous case didn't trigger.** Short single-sentence sources, blunt fabrications — Haiku
  handled them. Longer documents, subtler fabrications, or paraphrase-presence (where verbatim
  matching is the *wrong* check) would likely separate the conditions and are the obvious follow-up.
- N=13, synthetic, single-sample latency; `rg -F -i` (case-insensitive fixed-string) — whitespace/
  punctuation-variant "present" cases aren't tested.
- haiku+tool used one Haiku call to pick a fragment then a 0ms `rg`; a stricter "copy verbatim, no
  commentary" fragment prompt was needed even to reach 0.92 (the first prompt elicited refusals/
  commentary and scored 0.77 — the glue is prompt-sensitive).

## Bottom line

On a tool-coverable verification chore: **deterministic tool > whole-task model > model+tool.** The
decomposition payoff lives entirely in the *uncovered* residual; for the covered fraction the honest
move is to drop the model. This is the cleanest evidence yet for crystal's deterministic-default,
and a caution against reflexively inserting a cheap model where a tool already suffices.
