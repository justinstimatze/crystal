# A5 probe — the local cheap tier (`crystal local-probe`, 2026-05-29)

A5 is the sovereignty rung: swap the residual's cheap tier from cloud Haiku to a LOCAL model on owned
hardware. Before building the full slice, one probe — the way `probe` de-risked the cloud tier — asks
the question A5 turns on: **can a local small model do the categorize chore well enough, and fast
enough, to be the cheap tier — and, by the same accuracy number, to be the live ORACLE the `hook-loop`
re-author still lacks?**

Measured on the COVERED fraction (37 commands the deterministic rules cover, so `detClassify` is
ground truth — the only place with trustworthy labels). All numbers verified against the raw
`--verbose` rows by independent recount (37 rows, haiku✓=28, local✓=17).

## Result (this host: CPU, NO GPU — the brief's RTX 3080 is not reachable here)

| tier | accuracy vs det reference | latency p50 | latency p99 |
|---|---|---|---|
| deterministic (`detClassify`) | **37/37 = 1.00** (IS the reference — *after* the bug fix below) | ~µs | ~µs |
| cloud cheap (Haiku) | **29/37 = 0.78** (was 0.76 against the buggy reference) | 627ms | 1102ms |
| **LOCAL (`qwen2:1.5b`)** | **17/37 = 0.46** | **2470ms** | **16299ms** (one outlier 25s) |

## Verdict: the naive local tier does NOT pay on this hardware — a clean negative

- **Not viable as a cheap tier.** `qwen2:1.5b` loses on BOTH axes: accuracy 0.46 (worse than a coin
  flip across 8 classes is ~0.125, so it's learning *something*, but it's far below Haiku's 0.76 and
  the deterministic 1.00) AND latency (p50 2.5s vs Haiku 0.6s — ~4× SLOWER, because it runs on CPU
  with no GPU). The shift-left value prop (cheaper/faster behind a gate) inverts here: local is both
  worse and slower.
- **Too weak as a live oracle (0.46).** This was the real prize — a local label source would let the
  `hook-loop` re-author discover a new class's ground truth without the provided `containerRef` oracle
  (the gap [[hook-loop-closes-the-seam]] left open). At 0.46 it would feed the gate bad training
  labels half the time; the gate would (correctly) reject the re-authored table, or worse, promote a
  table trained on noise. **The no-live-oracle gap stays open.**

## The bonus finding — AUDITED (the ninth manufactured-confidence catch, self-caught)

The first draft of this doc claimed Haiku's disagreements with det were "genuine compound-command
ambiguity, not pure error." That was a fluent *vibes* claim — asserted, not inspected. The slimemold
reasoning hook flagged it as load-bearing-vibes, so I audited all 9 Haiku misses against the raw rows.
The audit overturned part of my own claim:

- **8 of 9 Haiku misses are det-correct** — `mkdir`→file-edit, `chmod`→file-edit, bare `cd`→nav,
  and the genuine compound/`cd`-reduce cases (`cd X && <action>` where det's reduce wins and the model
  is fooled by the leading `cd`). So the ambiguity story holds for most, but as *model error against a
  correct rule*, not symmetric ambiguity.
- **1 of 9 was a `detClassify` BUG, not ambiguity.** `until curl … | grep -q 200; do …; done; … && tail …`
  was labeled `search/inspect` because det inspects only each segment's LEADING token, and the `until`
  keyword masked the `curl` — so det landed on the `grep`/`tail` segments. The right label is `network`
  (a URL poll), and Haiku had said exactly that — it was marked *wrong against a buggy reference*.

**So "det = 1.00 ground truth" was contaminated by 1/37 (2.7%).** Fixed: `segClassify` now strips
leading shell control-flow / timing wrappers (`until`/`while`/`time`) that hide the action word — the
same bug family as the original leading-`cd` compound fix, found the same way (auditing real data),
excluding sudo/env/xargs (they interpose flag-args). Regression test in `cmd/triage_test.go`. After
the fix the reference is corrected and **Haiku rises 0.76 → 0.78** (its `until curl`→network call now
agrees with det); local stays 0.46.

The thesis direction survives the correction, now on a *clean* reference: **on its covered fraction the
deterministic tier is the MOST ACCURATE tier, not just the fastest** (1.00 > 0.78 > 0.46). Cheap tiers
earn their place only on the *residual* the rules can't cover; inside coverage, the corrected rule
table dominates. "Don't reach for a model where a rule is exact" — but *do* audit the rule against raw,
because the reference itself can carry a bug that silently mismeasures every tier compared to it.

## What this de-risks, and the path to a viable A5

The probe did its job: it showed the **naive** local tier (a 1.5B model, CPU, off-the-shelf, no
fine-tuning) is not viable — *before* the cost of building the full slice around it. A viable A5 needs
at least one of:
- **A GPU** (the brief's RTX 3080, not on this host) — would cut the 2.5s p50 toward Haiku's range or
  below; latency is a hardware problem, solvable.
- **A stronger local model** (7B–8B class) — likely closes much of the 0.46→0.76 accuracy gap;
  feasible on a 10GB GPU.
- **+LoRA fine-tuning on THIS chore** — the brief's actual bet; a tuned small model could match the
  deterministic reference on covered commands and propose trustworthy labels on the residual.
- **A confirm step** — treat a weak local oracle's labels as *proposals* a human or a cloud call
  ratifies before they train the gate (cheaper than full cloud, safer than trusting 0.46).

Until then, the cost gradient's far end is established as *aspirational on this hardware*: local
inference exists and is wired (`internal/local`, ollama), but a small CPU model is the wrong end of
the accuracy/latency frontier for this chore. The honest status: **A5 is plumbed and measured, not
yet paying** — the sovereignty pitch needs the GPU + a stronger/tuned model the brief assumed.
