# Substrate Crystallizability Measurement

> ⚠️ **CORRECTION (2026-05-28): the numbers below are RETRACTED.** The headline
> "crystallizable" pattern — `gt deacon heartbeat 2>&1`, reported at N=219, det=1.00 —
> is an artifact. The phrase `gt deacon heartbeat` appears ~5× in the real (non-self)
> transcripts and **zero** times as a `"command":` field; it lives inside a *persona
> prompt* ("I am Deacon. Start patrol: run gt deacon heartbeat…"). 219 ≫ 5 is a flat
> contradiction, so the measure aggregation is miscounting/mis-attributing. The
> transcript walker passes a synthetic regression test (`transcript_test.go`), so the
> bug is at the aggregation level or a real-data shape not yet reproduced; root-cause
> deferred (needs an instrumented scan). **Do not cite these counts as findings.**
> This is itself an instance of the failure mode the project exists to prevent: a
> fluent, confidently-quoted, plausible-but-wrong result, caught only by checking the
> raw source. See `THESIS.md` for the framing that supersedes this whole exercise.

Run 2026-05-28 via `crystal measure --home /home/justin --home /home/gas6amus`
over **217,184** registered-tool records from 160 main sessions (subagents excluded).

A pattern is *crystallizable* when an input-signature group is both **frequent**
(N ≥ 30) and **deterministic** (its outputs collapse to one comparator-equivalence
class, ≥ 0.95). Determinism is measured with `compare.Fingerprint` — the same
notion of "same output" the eval promote gate uses.

## Result

| granularity | groups | N≥30 | crystallizable |
|---|---|---|---|
| exact (full args) | 137,126 | 43 | **1** — `gt deacon heartbeat` (N=219, det=1.00) |
| normalized (program+arg / ext) | 5,754 | 312 | **2** — `until !` (N=54, 0.96), `pkill -9` (N=44, 0.98) |
| tool | 5 | 5 | 0 |

Out of 217k calls, ~3 patterns are frequent AND deterministic.

## Why — the frequency↔variance tension, quantified

High-frequency "real work" is uncrystallizable because its output depends on
mutable state the signature can't capture:

```
make t          N=427  det=0.04  (198 distinct outputs)
git push        N=240  det=0.02  (164 distinct)
git status      N=180  det=0.03  (115 distinct)
go build …gemot N=144  det=0.94  (4 distinct)   ← near-miss
Read:ext:.py    N=21313 det=0.00 (16,790 distinct)  ← looseness buys no determinism
```

Loosening the signature (`normalized`) exploded frequency but drove determinism
to ~0. You cannot buy crystallizability with looseness.

## Refined thesis (corrects the brief)

The brief assumed the residue is the high-frequency Bash tail ("Bash ~50% of
calls"). Empirically, the residue is the narrow class of **state-independent,
constant-output commands** (heartbeats, kills, no-output guards). State-dependent
commands dominate frequency and never reproduce.

## Implications for Phase 3 (GATE)

1. **GATE's primary filter is determinism, not frequency.** A frequency-first
   detector surfaces `make t`/`git push` and wastes the proposer. `measure`
   already ranks by determinism — keep it.
2. **Modal-hook opportunity at det 0.90–0.95** (`go build …gemot`: 4 distinct
   outputs): serve the modal output, defer to frontier on divergence. Needs the
   live drift detector — Phase 4+, not now.
3. **The target is small** — a handful of crystallizable command-templates per
   substrate. Honest input to whether the effort clears its own bar.

## Perf note

4m38s wall, 3.5 GB RSS (streaming accumulator; group-sample strings could be
truncated to shave RSS). One-shot analysis; not a hot path.
