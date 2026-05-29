# Topology Sim — testing the recursive thesis's riskiest assumption

**Riskiest assumption:** a stacked, self-reauthoring tier lattice (Opus authors
Sonnet's harness authors Haiku's…, drift escalates up, the upper tier re-authors)
stays *convergent* — failures get corrected, not silently absorbed. This is the
part publicrecord leaves to a human; it's the undercity nightmare multiplied per
layer.

**Cheapest honest test (no API):** model the feedback *topology* deterministically.
The only thing that varies is structure — there are no model-quality effects at all.
The anti-rigging guard is per-hop signal loss λ: the top sees `err · (1−λ)^(depth−1)`,
and can only re-author what it perceives. Without λ the loop trivially succeeds and
proves nothing; with λ the question is whether the control loop still stabilizes.

## Result (`crystal lattice`, drift=0.5, gain=0.5, demote>0.08, recover≤0.10)

```
depth\λ   0.0   0.1   0.2   0.3   0.4   0.5
1          ok    ok    ok    ok    ok    ok
2          ok    ok    ok    ok   res   res
3          ok    ok   res   res   res   res
4          ok    ok   res   res   res   SIL
5          ok   res   res   res   SIL   SIL
6          ok   res   res   res   SIL   SIL

max safe depth:  λ0.0→6   λ0.1→4   λ0.2→2   λ0.3→2   λ0.4→1   λ0.5→1
```

`ok` = recovers · `res` = alarms but stalls at a silent floor it can't see below ·
`SIL` = top never even alarms (fully silent degradation).

## What it says about the thesis

1. **The recursion is viable but SHALLOW, and the depth collapses fast with loss.**
   Lossless, you can stack arbitrarily deep — but that's fantasy. At a *modest* 20%
   per-hop signal loss, **max safe depth is 2.** The deep "Opus→Sonnet→Haiku→…" dream
   silently degrades unless up-propagation is near-lossless.

2. **Two distinct failure modes, both bad, emerge from topology alone** — before any
   model is involved: `res` (the top alarms but the signal is too attenuated to drive
   a full fix, leaving a residual error floor ≈ `demote/fidelity` it can't perceive)
   and `SIL` (fidelity so low the top never alarms — the pure undercity failure).

3. **publicrecord sits right at the viable frontier — and the human is the
   low-loss channel.** Its ~2–3 hand-authored tiers work *because* a person reads
   rich signals and edits scripts (near-λ0). Crystal's bet — removing that human —
   only survives if the up-channel is engineered for very low loss.

## The concrete design constraint this hands the next phase

The load-bearing variable is **λ, the per-hop information loss of the up-signal.**
So the live experiment shouldn't chase depth; it should attack λ: propagate
*structured, machine-checkable drift evidence* up the stack (the eval-gate's typed
divergences — `tool_use_id`, reason, fidelity), **not** natural-language summaries,
which is exactly where loss creeps in. Measure the real λ between two live tiers
before stacking a third. If live λ is high, stay at depth 2 (publicrecord's proven
regime) and invest in the signal channel, not more layers.

## Caveat (don't oversell)

This is a *necessary-condition* test of one structural property — that a lossy
control loop can stabilize. It does **not** show real models can author or re-author
correct harnesses (that's the live test). A pass here doesn't validate crystal; a
fail would have *invalidated* the deep version cheaply. It did: deep is out unless λ
is driven down.
