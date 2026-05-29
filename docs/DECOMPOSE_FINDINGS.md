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

### "Use the tool alone" smuggles in a competent caller (the load-bearing caveat)

`det-tool`'s 1.00 @ ~0ms is *not* a free lunch — the experiment handed `rg` the full quote as the
pattern with the right flags (`-F -i`). Something has to *decide* the invocation: which tool, what
pattern, fixed-vs-regex, case-sensitivity, how to interpret the hits. That arg/flag construction is
itself the glue — and it's exactly what `haiku+tool` fumbled (it chose a fragment that dropped the
distinguishing "43%"). So the honest reading of the three conditions:
- **det-tool** = the *post-crystallization steady state*: it works because a correct invocation was
  already authored. The authoring intelligence is real; it's just **spent once and amortized** over
  the recurring chore (= `crystallize` / partial evaluation). Its ~0ms is the *runtime* cost, not
  the total cost.
- **haiku+tool** = paying that authoring intelligence *per call* with a cheap model — which fumbles
  arg construction. Evidence you should author the invocation once, not re-derive it every call.
- For a genuine **one-off** chore (no recurrence to amortize over), you can't skip the arg
  intelligence — and a cheap model may get it wrong, so a one-off may justify the frontier tier.

This *is* crystal's thesis sharpened: the expensive tier's durable job is **authoring the correct
tool invocation** for a recurring chore; once authored, it runs deterministic and cheap. "Drop the
model" means "drop it from the per-call loop," not "no intelligence was ever required."

**But the invocation is usually dynamic + contextual — so the caller is usually a (cheap) LLM, not
a static wrapper.** The fully-deterministic `det-tool` case holds *only when the invocation is fixed
across the chore* (always "grep the full quote, literal, case-insensitive"). That's the minority —
the constant case crystallizes to a wrapper with no per-call model. The common case: the right
pattern/flags depend on *this* input, so the per-call caller has to be a cheap LLM. And a cheap LLM
caller **fumbles arg construction** (measured: it dropped the "43%"). So the general architecture is
three parts, not two: **cheap-LLM caller (dynamic invocation) → robust tool (execution) → cheap
*deterministic* verifier on the caller's output** (e.g. "is the chosen fragment a verbatim substring
of the claim, preserving its distinguishing tokens?" — which would have caught the dropped number).
The producer-verifier logic recurses onto the glue: a cheap caller is safe *to the extent its
invocation is cheaply checkable.* When it isn't, you need a smarter caller. So the real design
question per chore is **fixed-vs-contextual invocation** (crystallize the fixed ones; gate the
contextual cheap-LLM ones), and the verifiable-fraction is the master variable one level down too.

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
