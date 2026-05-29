# Depth sweep — does content-bearing channel loss compound over depth? (2026-05-29)

> **CORRECTION (see [CONTENTSWEEP_FINDINGS.md](CONTENTSWEEP_FINDINGS.md)).** This doc's claim that
> corrective content "erodes/inverts with depth" was a **misread**. The rigorous content-fidelity
> sweep shows content fidelity is **flat with depth (~0.7), not declining** — the failures it cites
> (items 5, 7, 10, 12) are present at **depth 1** and stable, not depth-progressive. Item 5 is
> inverted from d1, not "by d3." The correct conclusion: neither detection nor content fidelity
> compounds-loses to depth 6; the loss is at the **first hop**. Read the section below with that
> correction in mind.

ground-hop and uncover-hop measured per-hop loss λ≈0 at depth 1. The lattice's pessimism (safe
depth ~2) and STACKING #7's "a prose summary loses fidelity at every hop" both rest on loss
**compounding** as the signal is relayed up. This sweep relays uncover-hop's content-bearing prose
channel up through k paraphrase hops (a cheap tier rewrites the prior report in its own words,
instructed to preserve the specific claim) and measures, at each depth, whether the supervisor
still catches the tier-2 uncovered semantic error — plus a false-alarm rate on faithful items.

## Headline table (valid: 0 parse failures across all depths)

`crystal depth-sweep --verbose`, 14 tier-2 + 14 faithful, depth 6.

```
depth | R_tier2 (recall on uncovered drift) | FP_faithful | parse-fail
   1  | 14/14 = 1.00                        |  0/14       | 0
   2  | 14/14 = 1.00                        |  0/14       | 0
   3  | 14/14 = 1.00                        |  0/14       | 0
   4  | 14/14 = 1.00                        |  0/14       | 0
   5  | 14/14 = 1.00                        |  0/14       | 0
   6  | 14/14 = 1.00                        |  0/14       | 0
```

## The table OVERSTATES robustness — the metric is too coarse (read the chains)

The flat 1.00 is real but it measures only **detection** (did the supervisor flag *an* error). The
`--verbose` relay chains show the **corrective content** — which field is wrong, and the correct
value — is imperfect at depth 1 and visibly erodes with depth, while the binary metric scores all
of it as "caught."

- **Content is wrong even at depth 1 for some items.** Item 5: the extraction corrupted *name*
  (Mara Vega for gold Tomás Vega), but the depth-1 diff says *"name and role are wrong"* — the role
  was correct. Item 7: the *name* was the error; the diff says *"role wrong."* Correct detection,
  wrong diagnosis, at hop one.
- **Content erodes / inverts with depth.** Item 5 inverts by d3: *"Name should be Mara Vega"* — now
  asserting the WRONG value as the fix. Item 2 drifts from *"the extraction is wrong"* (d1) to
  *"COO change at Haulwell: Marcus Lee replacing Dana Pike"* (d6) — an event description, no longer
  an error flag.
- **All of these still score `v=false` (caught)** because the relayed report keeps signalling
  *something is off*. The binary recall cannot distinguish "correctly diagnosed" from "vaguely
  alarmed."

## The honest finding

**Repeated lossy relay decays rich content into a bare alarm bit.** Across depth, the channel
converges from "field X is wrong, the correct value is Y" toward "something here is wrong." At
depth 6 the alarm bit still fires (detection recall flat at 1.00), but the content a supervisor
would need to *act* correctly is degraded or inverted. Depth is the mechanism that turns content
into a verdict — which is exactly the content→verdict decay that
[content-not-format-predicts-channel-loss] predicted.

So the compounding question splits:
- **Error DETECTION** (1 bit) is robust to depth 6 for a content-preserving relay — STACKING #7's
  worry is *not* supported for detection.
- **Corrective-content FIDELITY** is *not* robust — imperfect at depth 1, eroding/inverting by
  depth 3 in some items — STACKING #7's worry *is* supported for content. The lattice's
  compounding pessimism is right about the thing that matters (acting correctly), wrong about the
  thing that's easy (noticing a problem).

## Caveats (these bound the claim — do not overstate)

- **The binary metric is the wrong instrument for the real question.** R_tier2 measures detection,
  not corrective fidelity. The rigorous refinement is a **content-fidelity metric**: does the
  depth-d report still name the *correct* value (Y), not just flag an error? That curve would
  quantify the erosion the chains show qualitatively. Until it's built, the compounding claim for
  content is qualitative (specific examples), not a measured curve.
- **Cooperative, instructed relay.** Each hop was told to preserve the specific claim. This is the
  *best-case* relay's decay curve. A non-preserving, harder-compressed (e.g. 5-word), or
  adversarial relay would erode detection too, and sooner.
- **Single cheap tier (Haiku) at every hop; N=14; depth 6.** Different relay models, larger N, or
  greater depth could move both curves.

## Bottom line (the three-hop arc)

- g is grounded (ground-hop: 1.00 byte-exact; uncover-hop: schema 0.00 / grounded 0.50 — the
  checkable fraction).
- Per-hop λ for **detection** is ~0 and stays ~0 to depth 6 (this sweep).
- Per-hop λ for **corrective content** is >0 — imperfect at depth 1 and compounding — but is so far
  only shown qualitatively. Quantifying it (the content-fidelity sweep) is the one remaining
  measurement before the lattice's depth-N claim is fully grounded rather than assumed.

The real, defensible thesis after three hops: **route content not verdicts (it's near-lossless for
one hop), but content itself decays toward a verdict under repeated relay — so depth, not format,
is where the lattice's pessimism actually lives.**
