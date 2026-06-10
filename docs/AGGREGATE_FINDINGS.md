# Aggregate — the first clean decomposition win (2026-05-29)

Hunting the cheap-model limit, found a different (better) result: **semantic aggregation** — "how
many of these N items match a criterion?" — is where decomposition decisively pays. Map-reduce
(cheap model classifies each item in isolation; deterministic code counts) beat *both* whole-task
models, including frontier Opus.

## Results (6 count-tasks, 8 items each; map verdicts verified item-by-item vs labels)

```
condition         exact-count   mean|err|   latency
opus-whole          3/6          0.50        1547 ms
haiku-whole         4/6          0.33         889 ms
haiku-mapreduce     6/6          0.00        6212 ms (8 sequential calls; parallelizable to ~1)
```

## What it shows

1. **Whole-task counting is a real weakness — for BOTH tiers.** opus-whole 3/6, haiku-whole 4/6;
   both miscount when classifying-and-counting over 8 items in one pass. Notably the *frontier* did
   no better (worse, on this small N) — so the fix is **not** "use a bigger model."
2. **Map-reduce is exact (6/6), and item-level perfect.** The per-item map verdicts matched the hard
   labels on **48/48** items across all six tasks (verified against the bitstrings, not just the
   totals). The cheap model is *excellent* at the focused per-item judgment; what it (and Opus)
   fumbles is the aggregation.
3. **The clean lesson:** *don't make a model count.* Decompose to per-item classification (the
   irreducible semantic judgment — cheap model nails it) + a **deterministic reduce** (the count —
   exact, free). This is the sharpest instance of the whole thesis: offload the mechanical part
   (aggregation) to deterministic code; spend the model only on the per-item residual. And the
   winning tier is *Haiku*, not Opus — shift-left + decompose beats the frontier here.
4. **Cost is the tradeoff.** Map-reduce ran 8 sequential calls (6212 ms) vs one whole-task call —
   but the calls are tiny/cheap and **embarrassingly parallel** (→ ~one-call latency), and each is
   Haiku not Opus. So the accuracy win is essentially free of a latency penalty once parallelized.

## Caveats

- N=6 count-tasks, 8 items each, single-sample latency. The per-item map was 48/48 — these items are
  fairly clean-cut; harder/ambiguous items would induce map errors, and *then* the question becomes
  whether a cheap per-item classifier holds (the `support --hard` result suggests it often does).
- Latency reported sequential; parallelization is the obvious fix and not yet implemented.
- This is `decompose`'s mirror image: there, decomposition *lost* (tool fully covered the chore, the
  glue fumbled); here it *wins* (the model owns the per-item judgment, deterministic code owns the
  aggregation the model is bad at). The unifying rule holds: **put each sub-step on the mechanism
  that's best at it** — per-item semantics on the cheap model, counting on deterministic code.

## Bottom line

The first decisive "decomposition pays" result, and it inverts the naive intuition twice: the
*frontier* model is not better at aggregation (don't pay up for it), and the *cheap* model + a
deterministic reduce beats both. Aggregation/counting belongs in code; the model should only ever
do the per-item semantic call. Map-reduce with a deterministic reduce is the pattern.
