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
| deterministic (`detClassify`) | **37/37 = 1.00** (IS the reference) | ~µs | ~µs |
| cloud cheap (Haiku) | **28/37 = 0.76** | 627ms | 1102ms |
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

## The bonus finding (reinforces the whole thesis)

Even **Haiku matches the deterministic reference only 0.76** on the covered fraction — the cloud cheap
model disagrees with the rules ~24% of the time on commands the rules cover. Part of that is genuine
ambiguity, not pure model error: the disagreements cluster on **compound commands** like
`cd X && git add …` (det calls it `git` via the "first real action beats a leading cd" reduce; the
models split it as `file-edit`/`nav`). But the direction is the thesis in one number: **on its covered
fraction the deterministic tier is not just faster than the cheap model — it is MORE ACCURATE** (1.00
vs 0.76 vs 0.46). The cheap tiers earn their place only on the *residual* the rules can't cover; inside
coverage, the rule table dominates. "Don't reach for a model where a rule is exact."

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
