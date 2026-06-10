# Self-authoring the verifier — `crystal author` (2026-05-29)

`triage` shipped the map-reduce + verifier stack with **hand-written** deterministic rules. `author`
closes the gap the handoff named: the **expensive tier authors the deterministic verifier itself**,
gated on a holdout, and **re-authors it when the input distribution drifts**. This is the actual
crystal mechanism — `triage`'s rules were the target shape, built by hand once; `author` generates
that target from data and proves the generated artifact is safe to serve only when it passes the gate.

## The loop (on the real Bash corpus)

1. **author** — Opus reads a representative SAMPLE of `(command, reference-label)` pairs and emits a
   deterministic rule table (`{match: prefix|contains, token, category}`) as JSON. The reference
   labels are the hand-authored `detClassify` verifier `triage` already validated — so the question
   is precisely *can the expensive tier author an equivalent deterministic classifier?*
2. **gate** — apply the authored rules to the full HELD-OUT set; promote only if accuracy vs the
   reference ≥ threshold (0.90). **No-verifier-no-crystallization, enforced.** The load-bearing
   negative control: a deliberately CORRUPTED rule table (every category rotated) must be REJECTED
   by the same gate.
3. **drift + re-author** — stream a NEW command class the rules never saw (containers:
   `docker`/`podman`/`kubectl`, correct category `container`, absent from the original set so the
   v1 rules *cannot* produce it). Windowed M-in-W demotion fires on the divergences → re-author WITH
   the new class in training → re-gate the drift class.

## Result on the live corpus (21,856 real Bash commands; 8,589 holdout)

```
author (800-example sample) → 78-rule deterministic table
gate:       authored   7990/8589 = 0.93  ≥ 0.90  → PROMOTE
            corrupted     89/8589 = 0.01          → REJECT   (negative control)
drift:      container class streamed → DEMOTE at index 2 (3-in-5 window); 3 leaked first
re-author:  83 rules → re-gate on the drift class 8/8 = 1.00 → RECOVERED
```

The gate cleanly separates a good authored artifact (0.93) from a broken one (0.01) — it is not an
always-promote rubber stamp, and it is not an always-reject. The full loop closes end to end:
**detect drift → re-author → re-pass.**

## The honest part: author fidelity scales with sample size, and the gate held at every step

The threshold was fixed at 0.90 throughout; the sample size was the only lever, and authoring
fidelity rose monotonically with it:

```
sample   200 → 0.87   REJECT
sample   400 → 0.89   REJECT
sample   800 → 0.93   PROMOTE
```

This is the result the discipline is *supposed* to produce. At 200 and 400 examples the expensive
tier authored a plausible, sensible-looking rule table (sample-200 had 64 well-formed rules:
`git→git`, `rg→search/inspect`, `mkdir→file-edit`, …) that nonetheless **disagreed with the
reference 11–13% of the time** — genuine miscategorizations (e.g. `wget→file-edit` where the
reference says `network`) plus compound-command ordering differences. The gate rejected those
plausible-but-imperfect artifacts rather than serving them. Only when given enough signal (800
examples) did the authored classifier reach reference fidelity and promote. **A green came from more
data, never from lowering the bar.**

The small (48-command corpus) run is the same lesson in miniature: a 19-command train sample is too
sparse for the tier to author complete coverage (`chmod`/`install`/`cp` had no examples → no rules
→ 0.78), and the gate correctly refused to crystallize it.

## What this proves (and doesn't)

- **Proves the self-authoring loop composes on real data**: the expensive tier authors a deterministic
  artifact, a deterministic gate decides promotion against a reference, the negative control confirms
  the gate is load-bearing, and a windowed-M-in-W drift trigger forces a verified re-author. The
  crystal mechanism — *expensive tier authors the cheap deterministic tier, gated, re-authored on
  drift* — runs end to end, not just as a hypothesis.
- **Doesn't prove** the served-hook/latency end (still a batch over a corpus, not a live PreToolUse
  hook in the loop — ROADMAP A1), nor that the *reference itself* is correct (the reference is the
  hand-rules; the loop measures fidelity-to-reference, not ground-truth-correctness — a fallible
  reference would propagate its errors, the same caveat `λ` carries).
- The drift class is a known-answer injection (a deterministic `container` reference), so the demote
  + recover is verifiable; a fuzzy real-world drift would need the cheap-model map step (as in
  `triage`) to even produce a comparand.

## Verification notes (house discipline)

- Raw authored JSON inspected directly (`--verbose`) before any accuracy number was trusted — the
  sample-200 table was read rule-by-rule and the per-holdout-command `applied/ref/match` rows were
  checked against the raw commands. The fidelity-scaling curve is three independent live runs, not
  one number extrapolated.
- Fail-loud parsing: an empty rule set, an unknown category, a bad match kind, or truncated JSON is
  an error, never a silent empty table that would auto-fail the gate. (The 800-sample run initially
  hit the 2048-token authoring cap → truncated JSON → fail-loud parse error; the cap, an impl limit
  not a gate, was raised to 8192.)
- LLM calls cached by content hash; the sweep above re-runs free.

## Bottom line

The verifier authors itself. The expensive tier writes the cheap deterministic classifier, the gate
admits it only at reference fidelity (0.93, not the plausible-but-wrong 0.87), the negative control
proves the gate has teeth, and an injected distribution shift drives a demotion and a verified
re-author that recovers fully. The loop the project was built to demonstrate now runs on real data.
