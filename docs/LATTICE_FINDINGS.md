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

> **CORRECTION (post adversarial-panel, 9 surviving findings).** An earlier version
> of this section claimed "**max safe depth is 2**" as a result. That number is
> **manufactured, not discovered** — it is contingent on three hand-set, unmeasured
> constants (`gain=0.5`, `demote=0.08`, `recover=0.10`), the same fluent-but-ungrounded
> failure class as the retracted 219-heartbeat count. The grid above is the `gain=0.5`
> corner. What survives is the *direction*, not the integer. Corrected below.

## What it says about the thesis

1. **The frontier is the algebra, not an emergent property.** The simulated frontier
   is the inequality `(1−λ)^(depth−1) ≥ demote/recover` restated (closed form
   `ClosedFormDepth` predicts 34/36 grid cells; the ±1 misses are discrete-overshoot
   at the boundary). The robust qualitative claim — **rising per-hop loss collapses
   the safe depth** — holds for every parameter setting tested. The specific integer
   does not.

2. **The integer is contingent on gain AND demote/recover, not just λ** (panel,
   verified in `lattice_test.go`):
   - **Gain flips it:** depth 3 / λ=0.2 *fails* at gain 0.5 but *converges* at 0.9 →
     max safe depth 2→3→4.
   - **Demote flips it:** at λ=0.2, demote 0.05→depth 4, 0.08→2, 0.09→1.
   So the earlier "**λ is THE load-bearing variable**" was **false**: gain and
   demote/recover are co-equal. Report the frontier as a **band** over the
   ungrounded knobs, never a single number.

3. **Three failure modes, all expressible now** (the over-correction one was hidden by
   an `err≥0` clamp the panel flagged; the clamp is removed):
   `residual` (alarms but stalls at a silent floor), `silent` (fidelity so low the top
   never alarms — pure undercity), and `unstable` (an over-eager re-author over-corrects
   and oscillates/diverges — the over-eager-fixes-a-working-harness failure).

4. **publicrecord's human is plausibly a low-loss up-channel** — but this is a
   *hypothesis the sim cannot confirm*, because **λ is never measured**; it is a swept
   knob. Every depth claim is therefore **conditional**: *IF λ≈X and the knobs sit at
   Y THEN depth≤Z.*

## The concrete design constraint this hands the next phase

The recommendation survives, restated honestly: **measure the real λ between two live
tiers before stacking a third**, and attack λ rather than chasing depth — propagate the
eval-gate's *structured typed divergences* (`tool_use_id`, reason, fidelity) up the
stack, not natural-language summaries. But λ is one of three unmeasured knobs, not the
sole lever; the live experiment must also pin the effective correction-gain and
detection threshold of a real re-author before any depth number means anything.

## Caveat (don't oversell — expanded per panel)

- This is a *necessary-condition* test of one structural property and is now known to be
  **the closed-form inequality `(1−λ)^(d−1) ≥ demote/recover`** plus a small discrete
  correction — useful for collapsing a vague fear into a falsifiable form, not for
  producing a real max-safe-depth.
- It **cannot** produce a grounded depth (λ unmeasured), cannot claim any depth is
  structural (it's gain/demote-contingent), and — even with the clamp removed — models
  drift as a single monotone scalar, a strawman for real multi-dimensional or
  false-alarm-injecting drift.
- It does **not** show real models can author/re-author correct harnesses (that's the
  live test). A pass does not validate crystal. The honest takeaway: *loss collapses
  safe depth — go measure λ, gain, and threshold on a real 2-tier boundary.*
