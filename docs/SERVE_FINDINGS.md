# Serving the deterministic tier — the payoff, measured (`crystal serve`, 2026-05-29)

Every result before this measured the *safety discipline* (does the gate catch errors, does the
signal survive, can the tier author the verifier). None showed shift-left *bought* anything. `serve`
measures the value prop directly: serve the deterministic tier (`detClassify`, the shipped hand
rules) in place of the cheap-model call on the fraction it covers, and report latency before vs
after, determinism, and the model round-trip removed — against the real cheap-model baseline.

## Result on the real corpus (48 Bash commands)

```
coverage:   deterministic tier covers 37/48 = g 0.77 (model handles the 11-command residual)

per-call latency (measured):
  deterministic classify:  ~7µs/call   (timed over 1000×48 calls)
  cheap model (Haiku):     p50 640ms, p99 1523ms   (real, persisted in cache)
  speedup on the covered fraction: ~90,000×

blended pipeline latency (serve-det-first vs all-model):
  before (model on all 48):         30,720 ms
  after  (det covers 37, model 11):  7,040 ms
  latency removed: 77%  (≈ g; the model call is deleted on the covered fraction, losslessly)

determinism:
  deterministic tier: exact-repro ✓ (identical over 2 runs)
  cheap model: NOT measured (cache makes reruns identical; would need live re-sampling)
```

## What it shows

- **The payoff is real and large on the covered fraction**: the deterministic tier deletes the model
  round-trip entirely (~7µs vs 640ms) at **zero quality cost** — because the rule *is* the reference
  answer on what it covers (this is not the `payoff` experiment's lossy gate; there is no quality
  trade on the covered fraction, only coverage).
- **Coverage g is the lever, and the residual is the binding constraint.** Blended latency drops by
  exactly g (77%); the remaining 23% still pays the model. Raising g (richer rules, more tool
  inventory, `author` re-authoring) is what moves the number — consistent with the whole shift-left
  picture.
- **Determinism is asserted, not assumed**: the served tier is byte-identical across runs. The
  model tier's determinism is honestly *not* claimed here — the cache masks it.

## Caching is the floor of shift-left ("stupid cache tricks")

`serve`'s baseline latencies are *persisted real measurements*: `.crystal-cache` stores each
completion's text **and** its measured wall-clock, so a repeat input replays the answer (and the
710ms it once cost) from disk in microseconds — the model round-trip deleted. A cache hit is the
cheapest shift-left of all: the degenerate floor of the substrate gradient (maximal determinism,
zero generalization). The cheap end is three rungs — **cache** (exact-repeat inputs) → **rule table**
(a recurring *pattern*, generalizes past the seen set) → **cheap model** (the open residual). See
`THESIS.md` → "Memoization is the floor of the gradient."

## Verification notes (house discipline — and a near-miss)

- A first cache grep appeared to show Haiku latencies of 0–15ms, contradicting the reported p50 of
  640ms. **Held the number as unverified and reconciled it before trusting:** the `--verbose`
  per-command latencies (469–1819ms, p50 635) and a corrected count (259/437 Haiku entries carry
  real ≥100ms latencies; a sample entry: `"build/test"`, `latency_ms: 710`) confirmed the figure was
  real and the *grep pipeline* was broken (mangled arg list), not the data. The discipline caught a
  measurement-of-the-measurement bug, not an eighth manufactured number — but only because the number
  was not trusted on sight.
- Two consecutive `serve` runs each complete in ~0.8s wall (not the ~30s that 48 live Haiku calls
  would take), confirming the latencies are cache-replayed for free, exactly as designed.
- The deterministic per-call figure is timed over 1000×48 iterations (a single call is sub-µs noise).

## What this doesn't show (the honest gap that remains)

Still a **local microbenchmark, not a live PreToolUse hook** in a real Claude Code agentic loop. The
next rung (ROADMAP A1/A2) is to install a promoted artifact as an actual hook answering in place of
a frontier call, and measure end-to-end latency and the amortization point in the loop — including
the one-time authoring cost (`author`'s Opus call) amortized over served hits.

## Bottom line

Shift-left, served, pays: on the covered fraction the deterministic tier removes the model
round-trip entirely (~90,000× faster, exact-repro) at no quality cost, dropping blended pipeline
latency by g (77%). The residual is the binding constraint, so coverage is the lever — and the
cheapest rung of all, a cache hit, is the same move taken to its trivial floor.
