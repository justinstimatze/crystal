# Content-fidelity sweep — corrects the depth-sweep narrative (2026-05-29)

depth-sweep showed error DETECTION flat at 1.00 across depth and I read its verbose chains as
"corrective content erodes/inverts with depth." This sweep measures content fidelity directly and
**overturns that reading**: content fidelity is **flat with depth, not declining**, and the
errors are baked in at the first hop.

## Method

For each tier-2 item, relay the prose channel through k hops (the SAME chains as depth-sweep,
cache hits). At each depth a supervisor, **blind to ground truth**, recovers the proposed
correction `{field, value}` from the depth-d report. The recovery is scored against the hard
labels: `gold` (names the correct value — substring, or a narrow equivalence judge for synonyms
like CEO ≡ chief executive officer), `inverted` (names the WRONG/corrupted value as the fix —
deterministic), or `other`. content-fidelity(d) = field✓ AND value=gold.

## Results (valid: 0 parse failures)

`crystal content-sweep --verbose`, 14 tier-2 items, depth 6.

```
depth | field-acc | value=gold | inverted | other | content-fidelity | parse-fail
   1  |   10/14   |    9/14    |    1     |   4   |   9/14 = 0.64    | 0
   2  |   11/14   |   10/14    |    1     |   3   |  10/14 = 0.71    | 0
   3  |   10/14   |   10/14    |    0     |   4   |  10/14 = 0.71    | 0
   4  |   11/14   |   10/14    |    1     |   3   |  10/14 = 0.71    | 0
   5  |   11/14   |   10/14    |    1     |   3   |  10/14 = 0.71    | 0
   6  |   10/14   |    9/14    |    1     |   4   |   9/14 = 0.64    | 0
```

**Flat at ~0.64–0.71. No downward trend.** 8 items are correct at every depth; ~4 are wrong at
every depth; 1 (item 2) is noisy (right d2–d5, wrong d1/d6). The d1/d6 dips are that single
swing item, not erosion.

## What this corrects

The depth-sweep finding claimed content "erodes/inverts with depth," citing item 5 inverting "by
depth 3." That was a **misread of depth-1 errors as depth-progression**. The truth:

- **Item 5 is inverted from depth 1**, not by d3 (d3 was actually the odd one out, flipping to
  `other`). The inversion is a first-hop failure, stable thereafter.
- **Items 7, 10, 12 are wrong from d1 through d6, unchanged** — no accumulation.

So a content-preserving relay does **not** compound-lose detection OR content fidelity to depth 6.
The ceiling (~70%) is set at the **first hop** and held flat across six relays. The lattice's
compounding-over-depth pessimism is **not supported in this regime** (cooperative instructed
relay, N=14, depth 6, Haiku at every hop).

## The ~30% shortfall is at hop 1 — and is itself confounded

The persistent failures trace to the first hop, in two distinguishable ways visible in the chains:

- **Channel failure** (items 7, 10): the original Haiku diff latched onto the distractor's *real*
  attribute (Paul Adams genuinely is interim CEO; Tom Bradley is a junior engineer) instead of
  naming the correct subject — so the report never carried the right answer.
- **Recovery-reader failure** (item 12): the diff *did* carry it (`"Ines Roca is the COO"`), but
  the blind recovery step picked the wrong span (`board chair`).

These cannot be cleanly separated here — the recovery step is itself a fallible reader (the same
fallible-judge issue ground-hop flagged). Both are first-hop effects, not depth effects.

## Caveats

- Cooperative, instructed relay; Haiku at every hop; N=14; depth 6. A non-preserving / harder /
  adversarial relay, or larger N/depth, could change the picture — flatness here is not proof of
  flatness everywhere.
- content-fidelity conflates channel loss and recovery-reader loss. Disentangling them needs a
  reference that reads the report without the failure modes of a single Opus call (e.g. multiple
  independent readers, or a structured channel).
- The hard class is **name-distractor** corruptions where the distractor person has a distinct
  role in the sentence: the channel/reader tends to report that person's role rather than recover
  the correct subject. org/role swaps recovered cleanly (8/8 of those items stable-correct).

## Bottom line (revises the depth-sweep conclusion)

Across the full arc, the honest result is: **the loss is at hop 1, not in compounding over depth.**
- Format does not predict loss (content ≈ typed at one hop — uncover-hop).
- Depth does not compound it (detection AND content flat to depth 6 — depth-sweep + this sweep).
- What bounds fidelity is the **first hop**: the summarizer's diagnostic accuracy plus the reader's
  extraction (~70% here, lower on name-distractors). That first-hop ceiling — not depth, not
  format — is where the engineering effort belongs.
