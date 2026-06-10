# Minimal grounding hop — λ and g on the real corpus (2026-05-29)

This is the first *valid* live grounding of two of the lattice's assumed knobs — per-hop
signal loss **λ** and deterministic guardrail coverage **g** — on real transcript records.
It deliberately repairs the three defects that made the earlier `experiment` run
INSTRUMENT-INVALID (see [EXPERIMENT_FINDINGS.md](EXPERIMENT_FINDINGS.md)).

## The claim under test

`STACKING.md` #7 (lossless up-signal) asserts the one genuinely novel thing crystal can
say: **a deterministic typed up-channel loses less supervisor signal than a prose
up-channel.** That is falsifiable, and measuring it grounds λ on the real substrate.

## Method (pre-registered — written before the numbers)

- **Population.** Real `Bash` records from `testdata/corpus` (N≥30).
- **Ground-truth-by-construction labels.** Each record is either a faithful reproduction
  (identity) or a known injected corruption (`bash-dropline` / `bash-flipdigit` /
  `bash-flipexit`). A record is labeled `drift` **only when the corruptor actually fired**
  (`Mutated()==true`); otherwise it is `faithful`. Truth is defined by construction — not by
  a hand-written gold (the extraction run's defect) and not by a model's verdict.
- **One supervisor question, three up-channels, one bar.** "Is the worker output a faithful
  reproduction of the reference?"
  - **det** — crystal's registered comparator (`compare.Lookup`), no LLM. Typed verdict.
  - **prose** — Haiku writes a *concrete diff* (verifiable content, explicitly **not** a
    verdict); Opus judges faithful/drift from the prose alone.
  - **full** — Opus judges from the full reference + worker outputs.
  The prose and full judges use the **same model, same question, same bar** (`judgeBar`),
  differing only in how much signal reaches them.
- **Metrics.** g = det recall on injected drift. λ_channel = disagreement(full, channel).
  Each channel's accuracy/precision/recall vs the hard labels is also reported, so
  "disagreement" can be split into *lost signal* vs *more correct*.

### Why this fixes the three prior defects

1. **Labels are ground truth by construction**, not exact-match against paraphrased gold.
2. **The up-channel carries verifiable content** (a concrete diff / a typed record), never a
   self-graded verdict to rubber-stamp — the exact failure that produced the bogus λ=0.90.
3. **Same judge model + same question + same bar** across the full and prose channels.

## Pre-registered caveats (these hold regardless of the numbers)

- **λ_det ≈ 0 is EXPECTED and partly tautological.** For a byte-exact tool the deterministic
  comparator covers ~all injected corruptions (the eval go/no-go test already proves this
  sensitivity). So a near-zero λ_det is not a discovery — it is the comparator working as
  designed. **The result is the CONTRAST** (does the prose channel lose signal the typed
  channel keeps?), not the absolute λ_det.
- **The interesting λ is out of scope here.** The high-value loss is on *semantic / uncovered*
  drift — judgments a byte comparator cannot make — where the prose channel is the *only*
  available signal. That is the next experiment, not this one.
- **The full judge is a model, not an oracle.** Its own accuracy vs the hard labels is
  reported. If it is itself inaccurate, λ measured against it is correspondingly weakened.
- **One tool, one hop, synthetic corruptions.** This grounds depth-1 loss for byte-exact
  drift. It does not establish the depth-N convergence frontier the lattice models.

## Run 1 was INVALID — the fourth catch (and how it was caught)

The first `ground-hop` run reported `λ_det=0.72 / λ_prose=0.00` ("prose loses nothing"). The
`--verbose` dump showed **every** LLM verdict was `false` (DRIFT) — including the 36 faithful
rows whose prose literally said *"identical"*. Cause: the judge calls used `max_tokens=5`, and
Opus 4.8's adaptive thinking tokens count against that budget, so the model returned **empty
visible text**; the prefix-match parser then defaulted every verdict to DRIFT. λ_prose=0.00 was
an artifact of two *degenerate, identical* always-DRIFT classifiers agreeing.

Caught by the instrument's own pre-registered guardrails: the `--verbose` per-item dump and the
"report the reference judge's own accuracy vs labels" check (full acc=0.28 = a constant
classifier). This is the same manufactured-confidence pattern as 219 / lattice-depth / the
extraction run — the fourth time, and the first inside this command.

**Fix:** added `llm.Client.Classify` (thinking **disabled**, so a one-word verdict is never
starved), raised the judge budget to 16 tokens, and replaced the fragile prefix-match with
`parseVerdict` — which finds the keyword anywhere and treats empty/ambiguous output as an
explicit **parse failure** (surfaced loudly, excluded from λ), never a silent class.

## Results (run 2 — valid: 0 parse failures, reference judge 96% accurate vs labels)

`crystal ground-hop --verbose`, N=50 Bash records (14 injected-drift, 36 faithful). Every
aggregate below was confirmed against the raw per-item rows.

```
g (deterministic, no API): det recall on injected drift = 14/14 = 1.00   det acc=1.00 prec=1.00
λ_det   (full vs typed channel)   = 2/50 = 0.04
λ_prose (full vs prose channel)   = 1/50 = 0.02
accuracy vs hard labels:  det 1.00  |  prose 0.98 (recall 13/14)  |  full 0.96 (recall 12/14)
```

### What is solid

- **g = 1.00.** The deterministic comparator caught all 14 byte-level Bash corruptions with
  zero false positives. Pure Go, no model, consistent with the eval go/no-go test. This is the
  one fully trustworthy number.

### What the λ contrast actually shows (and what it does NOT)

- **The naive STACKING #7 prediction is NOT supported on this substrate.** "A deterministic
  up-channel loses much less than a prose up-channel" predicts λ_prose ≫ λ_det. Measured:
  λ_prose (0.02) ≤ λ_det (0.04), and the prose channel's accuracy vs labels (0.98) is within one
  record of the typed channel. For byte-exact Bash drift, the prose channel tracks the supervisor
  nearly as well as the typed channel. The thesis claim is, at best, **not visible here** —
  plausibly because this is exactly the regime (covered, byte-exact drift) where a prose
  description of the diff is easy and faithful.
- **λ-measured-against-the-full-judge is a contaminated metric, because the reference is
  fallible.** Both λ_det disagreements are cases where the *full judge erred*, not where the
  channel lost signal:
  - **Row 15** (`bash-flipdigit` on a git hash `556c9d9`): det and prose both correctly flagged
    DRIFT; the full judge dismissed it as a "volatile hex span" (obeying the judge instruction).
  - **Row 13** (`bash-dropline` on a >1500-char output): the dropped last line falls past the
    render truncation, so both LLM channels saw identical text and said faithful; det compared the
    full Output and caught it.
  So λ_det=0.04 mostly means "det was right and the reference was wrong twice," not "det lost
  signal." **Accuracy-vs-hard-labels (det 1.00 > prose 0.98 > full 0.96) is the honest lens; λ
  defined as disagreement-with-a-model is the wrong instrument when the model errs.** This is
  itself a methodological finding for the lattice: per-hop λ cannot be grounded against a fallible
  supervisor — it needs hard labels or a verified reference.
- **The real, narrower win for the deterministic channel** is precision-of-correctness: it is the
  only channel that is *exactly* right, catching the two cases that fooled the semantic judge (a
  volatile-looking hash; a truncation-hidden tail). That is a determinism/coverage argument, not a
  signal-loss-at-depth argument.

### Instrument limitations (disclosed, not hidden)

1. **Fallible reference.** The full-signal "reference" judge is itself a model (96% vs labels);
   λ-vs-reference inherits its errors. Mitigated by also reporting accuracy vs hard labels.
2. **Render truncation (1500 chars).** End-of-output drops on long outputs are invisible to the
   LLM channels (affected ≥1 row, #13), handicapping them on long-output `dropline` cases. Does
   not affect g (det reads the full Output). Fixable by diffing/eliding the middle instead of
   truncating the tail.
3. **One tool, one hop, synthetic byte-exact corruptions.** This grounds depth-1 coverage and the
   prose-vs-typed contrast for *covered* drift. It says nothing about the depth-N convergence
   frontier, and nothing about the high-value regime — *semantic / uncovered* drift a byte
   comparator cannot judge — which is where a prose channel would actually earn its keep. That is
   the next experiment.

### Bottom line

g is grounded (1.00 for byte-exact Bash drift). λ is **not** cleanly grounded: the byte-exact
regime is the wrong place to see channel loss, and disagreement-vs-a-model is the wrong metric
when the model errs. The next hop must target *uncovered semantic drift* with *hard labels or a
verified reference* — that is where the deterministic-vs-prose channel question is actually live.
