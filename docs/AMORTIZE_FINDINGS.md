# The amortization breakeven — `crystal amortize` (2026-05-29)

`serve` showed the per-hit latency win but ignored that a promoted artifact isn't free to *make* —
the expensive tier paid a one-time authoring round-trip. `amortize` prices it: how many served hits
repay that cost, and how often can drift force a re-author before the win is erased? All inputs are
already in the disk cache (the `author` call's real `LatencyMS` + tokens; the per-command model
latencies), so it reconstructs them for free.

## The accounting (pure wall-clock — the thesis axis)

```
baseline (no crystal):  N_covered × model_p50          call the model every covered hit
crystal:                T_author + N_covered × det     author once, then serve ~free
crystal wins once:      N_covered > T_author / (model_p50 − det)   ≡ the breakeven
```

## Result (live corpus, 800-example authored table)

```
one-time authoring (real, from cache):
  author call: 23,552 ms wall-clock (Opus, 800-example sample), 144,432 in / 2,437 out tokens
per-hit latency saved (covered command):
  ~548ms (Haiku p50) − 0.003ms (det) ≈ 548 ms saved/hit
breakeven:
  23,552ms / 548ms = 43 covered hits to repay authoring
  (corpus has 17,402 covered commands → already ~405× past breakeven)
```

**Latency repays in 43 hits.** The one-time 23.5s Opus authoring round-trip is paid back after 43
served covered commands — and a real personal corpus has thousands. (Verified against raw cache: an
Opus entry with `latency_ms: 23552, input_tokens: 144432` is the exact call priced.)

## The load-bearing half: the drift bound

The symmetric question is the dangerous one — drift forces re-authoring, and **re-authoring more
often than once per breakeven (43) covered hits nets NEGATIVE**:

```
net latency vs baseline at re-author cadence R (covered hits per re-author):
  R=10    baseline    5,480ms   crystal  23,552ms   net  −330%   LOSS
  R=43    baseline   23,564ms   crystal  23,552ms   net    +0%   WIN (breakeven)
  R=172   baseline   94,256ms   crystal  23,553ms   net   +75%   WIN
  R=860   baseline  471,280ms   crystal  23,555ms   net   +95%   WIN
```

This is **exactly why demote-on-drift — not just drift *detection*, and not trigger-happy
re-authoring — is load-bearing.** A gate that re-authors on every blip churns the artifact faster
than it amortizes and turns the win into a 330% loss. The value of the windowed M-in-W demotion
([`DRIFT_FINDINGS.md`]) is that it re-authors *only* on sustained drift, keeping the re-author
cadence well above breakeven. Authoring being cheap (23s) is not sufficient; authoring being *rare*
is what makes it pay.

## Token economics (reported second — the collapsing axis)

```
authoring: ~$0.78 (Opus, one call, 144k in / 2.4k out)
per covered hit saved: ~$0.000266 (a Haiku call, ~236 in / 6 out)
token breakeven: ~2,944 covered hits
```

The honest nuance, and it *supports* the thesis: **latency repays in 43 hits; tokens repay in
~2,944** — a ~70× longer payback. If your binding constraint were token cost, crystallizing this
chore is a much slower win. But the thesis is precisely that the binding constraint is *not* token
cost (collapsing) — it's latency, determinism, and sovereignty, where the payback is fast. The
numbers make the bitter-lesson framing concrete: **crystal is latency/determinism engineering, not
token-cost engineering** — and priced on the wrong axis it looks ~70× weaker.

## Verification notes (house discipline — two cost-discipline catches)

- Building this, an early version looped the cheap model over the *whole* 21k-command corpus for the
  latency baseline (tens of thousands of live calls) — caught and capped to a 60-command probe
  (~$0.016, then cached). A second version timed the deterministic tier 1000× over all 21k commands
  (~22M calls, ~150s) — capped to a 200-command sample (per-call latency is corpus-size-independent).
  Both are the cost-awareness rule applied to the *measurement*, not the product.
- The authoring latency and token counts are the real persisted values from the `author` run,
  confirmed against the raw Opus cache entry (`23552ms / 144432 tokens`), not a default.

## Bottom line

The authored verifier is cheap to make (one ~23s Opus call) and repays its latency in **43 served
hits** — trivially cleared by any recurring chore. The binding risk is not authoring cost but
**re-author churn**: drift that forces re-authoring faster than once per 43 hits erases the win,
which is why *demote-on-drift* (re-author only on sustained drift) is the load-bearing mechanism, not
detection alone. Priced on tokens the win is ~70× slower — which is the thesis, stated as a number:
the payoff lives on the latency/determinism axis, not the collapsing token axis.
