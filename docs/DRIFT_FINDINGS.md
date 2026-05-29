# Drift-Detection Experiment — testing the riskiest assumption

The project's load-bearing safety claim is that drift detection catches a
crystallized hook going wrong *before* it produces sustained silent-wrong
output (the brief's undercity nightmare). Phase 1 only proved the *comparator*
catches *synthetic* corruption — it said nothing about the *demotion rule*
against real, time-ordered drift. This experiment tests that.

Method: **temporal replay.** Promote a modal hook on a pattern's earliest
occurrences, stream the rest in timestamp order, serve the hook's output,
compare to what the frontier actually produced, and demote per a divergence
rule. Measure not just *whether* it demotes but how many wrong outputs **leak**
before it does.

## Finding 1 — the brief's consecutive-K rule is evadable (controlled)

`internal/drift/drift_test.go`:
- **stable pattern** → never demotes, 0 leaked (no false alarm). ✓
- **clean shift** (output changes and stays changed) → demotes within K, leak
  bounded by K (3). ✓ — this is the brief's intended success case.
- **intermittent / flapping drift** (alternating correct/wrong) → **never
  accumulates K in a row, so consecutive-K NEVER fires while 30 wrong outputs
  leak.** This is the undercity failure mode, reproduced.

The leak under consecutive-K is bounded by *drift cadence*, not by K.

## Finding 2 — corroborated on real time-ordered history

`crystal drift` over the full substrate (substring-matched patterns; coarser
than the exact crystallizable signature, so determinism is understated):

```
pattern              train det  stream  servedCorrect  leaked  demoted@
gt deacon heartbeat    0.90      162        47           8       idx 54
git status             0.01     1763         0           3       idx 2
git push               0.01     1644         0           3       idx 2
go build -o gemot      0.31      244         6           3       idx 8
```

- **Promote gate worked:** all four train det < 0.95 → none would have been
  deployed. Layer-1 trust holds.
- **Leak tracks cadence, as predicted:** fully-volatile patterns (git
  status/push) trip 3-in-a-row instantly (3 leaked); the near-stable heartbeat
  drifts intermittently and **leaked 8** before a persistent run finally fired.
  The closer to stable, the more leaks before consecutive-K catches it.

## The fix — sliding-window rate rule (verified)

Demote on **M divergences within a sliding window of the last W** outputs.
This generalizes consecutive-K (W==M==K is exactly "K in a row") but with W>M
it bounds leakage regardless of drift cadence.

`TestWindowedRuleCatchesIntermittentDrift`: the same intermittent drift that
consecutive-K let leak 30 → windowed rule (3-in-10) **demoted at index 5,
leaked only 3.** Available via `crystal drift --k 3 --window 10`.

## Takeaways

1. The eval/promote gate correctly refuses to deploy non-deterministic patterns
   — the first line of defense works on real data.
2. The brief's "demote on 3 consecutive divergences" is **insufficient**:
   intermittent drift evades it and leaks unbounded silent-wrong output.
   Replace with an M-in-W sliding-window rule.
3. Recommended next: re-run `crystal drift --window 10` on the real heartbeat
   pattern to confirm the real-data leak drops from 8; tune M/W against the
   acceptable silent-wrong budget (the "<24h" bound, expressed in calls).
