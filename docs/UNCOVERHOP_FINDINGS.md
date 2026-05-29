# Uncovered-drift hop — the g<1 regime, and depth-1 λ on the residual (2026-05-29)

ground-hop ([GROUNDHOP_FINDINGS.md](GROUNDHOP_FINDINGS.md)) grounded g for byte-exact drift
(g=1) but could not see channel loss: when the deterministic comparator covers everything,
there is no residual for a fuzzy channel to work on. This hop builds the complementary regime —
errors a deterministic check **provably cannot** catch (g<1) — and measures whether a fuzzy
channel recovers the residual and how much is lost across one prose-compression hop.

## Construction (hard labels by injection; verified before any tokens spent)

Task: extract `{name, role, org}` from a sentence. 14 source items, each yielding three
instances with **known** labels:

- **faithful** — the gold extraction.
- **tier-1** — one field swapped to an *unrelated* value not present in the source.
- **tier-2** — one field swapped to a *distractor that appears verbatim in the source* (a
  former employer, an outgoing officer, a different person named in the sentence).

`TestUncoverHopConstruction` asserts the invariants deterministically (gold fields grounded;
tier-2 distractor present-but-wrong; tier-1 value absent) — so a mislabeled item cannot silently
invalidate the run the way paraphrased gold invalidated `experiment`.

Two deterministic guardrails expose the **checkable fraction**:

- `det-schema` — valid, non-empty JSON.
- `det-grounded` — every extracted field must be a substring of the source.

The grounded per-hop loss **λ = R_full − R_prose** is measured on the tier-2 residual as recall
**against the hard labels** — never against a model verdict (the contaminated metric ground-hop
exposed).

## Results (valid: 0 parse failures, reference judge acc=1.00 vs labels)

`crystal uncover-hop --verbose`, N=42 (14 faithful, 14 tier-1, 14 tier-2). Every aggregate
confirmed against the per-item rows.

```
deterministic guardrail coverage g (no API):
  det-schema   (valid non-empty JSON):   g = 0/28  = 0.00   acc=0.33 prec=0.00
  det-grounded (every field in source):  g = 14/28 = 0.50   acc=0.67 prec=1.00   (catches all tier-1, misses all tier-2)

fuzzy recovery of the uncovered residual (tier-2, vs hard labels):
  R_full  (supervisor, full source) = 14/14 = 1.00
  R_prose (supervisor, prose diff)  = 14/14 = 1.00
  per-hop loss λ = R_full − R_prose = +0.00

overall accuracy vs hard labels: det-schema 0.33 | det-grounded 0.67 | prose 0.98 | full 1.00
```

## What this shows

1. **The "checkable fraction" is real and expands with check sophistication.** A schema check
   sees 0% of semantic drift; substring-grounding catches the 50% whose wrong value isn't in the
   source; the remaining 50% (distractor-in-source) is genuinely uncovered by *any* substring
   check. This is STACKING #7's "the checkable fraction is capped by what you can write a check
   for," demonstrated with a hard boundary.

2. **On the uncovered residual, a content-bearing prose channel lost nothing at one hop
   (λ=0.00).** Both the full-source supervisor and the prose-diff channel recovered 14/14 tier-2
   errors. The prose did genuine semantic work — *"Dana Pike is the outgoing COO,"* *"Tom Bradley
   is a junior engineer,"* *"Diego Ramos is board chair, not COO"* — and a competent judge read it
   losslessly. This is real recovery, not a degenerate always-DRIFT classifier (faithful rows are
   correctly passed: prose acc=0.98, full acc=1.00).

3. **This refines STACKING #7's lossless-up-signal claim.** #7 frames the axis as *typed/deterministic
   vs prose* ("a prose summary loses fidelity at every hop"). The two hops together say the real
   axis is **content vs verdict**, not format:
   - The λ=0.90 catastrophe (see GROUNDHOP run 1 and EXPERIMENT) was a *verdict-only* channel — the
     summary carried "looks correct," no evidence — and was maximally lossy.
   - A *content-bearing* prose diff (which field is wrong and why) was ~lossless here, just like a
     typed divergence record would be.
   A typed record is still easier to sanitize and to calibrate (#3, #6), but it is not the source
   of the fidelity advantage. **Route content, not verdicts** is the load-bearing rule.

## Caveats (these bound the claim)

- **Depth-1 only.** STACKING #7's actual worry is loss *compounding over depth* — a
  summary-of-a-summary-of-a-summary. λ=0 at one hop says nothing about λ at depth N. The
  depth-sweep (re-summarize the prose k times, measure recall vs k) is the experiment that would
  test the compounding claim, and neither hop touches it.
- **Easy end of semantic drift.** Every tier-2 distractor carries a textual cue (*former,
  outgoing, junior, previously*). Residual that needs world knowledge or multi-sentence inference,
  with no in-text cue, may well show λ>0. This is the catchable end of the uncovered space.
- **N=14 tier-2 — wide confidence interval.** λ=0.00 means "no loss detected at this N," not
  "proven zero." Do not quote it as a constant.
- **The prose channel is summarizer-bounded.** One Haiku summary truncated at the 60-token cap to
  garbage (`name="Sofia Russo" role="org="`) and the judge passed it — a tier-1 miss (prose overall
  recall 0.96 = 27/28). It does not touch the tier-2 λ, but a longer/harder source would truncate
  more often; the fuzzy channel is only as good as its summarizer's budget and competence.

## Bottom line (combined with ground-hop)

Per-hop signal loss is **~0 at depth 1** whether the up-channel is a typed comparator (covered
regime) or a content-bearing prose diff (uncovered regime). The lattice's assumed λ>0 is **not
validated at depth 1**, and the deterministic-vs-prose framing is the wrong cut — content-vs-verdict
is the real one. The genuinely open risk is **compounding over depth**, which needs a true
multi-hop summarize-of-summarize stack. That is the next experiment, and it is the one that would
actually move the lattice's depth-N convergence claim off "assumed."
