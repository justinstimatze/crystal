# Recipe ladder — what crystal actually sediments (working note)

Status: working note, not polished. A thesis sharpening from a 2026-06-30
session. The point: crystal does NOT sediment cached answers — it sediments
transferable recipes, and the recipe's specificity is the lever that lets a
weaker executor take over.

## The target is a recipe, not an answer

A cached answer is the degenerate extreme of executor-descent: zero execution,
but it only serves the EXACT repeat. (Funes: perfect memory of one particular,
useless the moment the instance varies.) What survives variation across a
recurring chore is the abstraction — the recipe. So the sediment is a
transferable procedure, authored at the lowest specificity a weaker executor can
still reliably run.

## The ladder

```
memory (the answer)   ← degenerate: no execution, exact-repeat only
   ▲ more transferable / weaker executor suffices
plan                  ← high-level; needs a fairly strong model
recipe                ← stepwise + some judgment; ~Haiku-executable
pseudocode            ← near-mechanical; a very weak / local-open model
real code             ← deterministic; NO model (the code tier)
```

## The load-bearing claim

Recipe-specificity sets the EXECUTOR FLOOR. How specific Opus makes the artifact
decides how weak a model can still execute it. So the ladder is the lever for the
menu descent: push the artifact down a rung → the work it carries moves down the
executor axis (cheaper); if that cheaper executor runs on your own hardware /
open weights, down placement and openness too.

(Axis precision, corrected in the same session: placement = where the COMPUTE
runs, openness = which MODEL. A local memory file is neither — it's a
cheaper-executor artifact. The all-three corner is reached either by deterministic
code, OR by a local open-weight model running the recipe.)

"More and more taken from Opus" = ratchet the artifact down a rung over
iterations so a weaker executor takes a growing TRANSFER FRACTION.

## Where this re-centers what's built

- `author-fn` + the dogfood generator are the BOTTOM rung — real code,
  deterministic, no model. We've been at the all-the-way-down end (the easy end:
  code recipe-ables all the way to deterministic).
- The unexplored rich MIDDLE is recipe/pseudocode executed by a weaker MODEL —
  the local-open-weight tier RUNS the recipe. The gate generalizes from
  "does it compile" to "does (weak model + Opus's recipe) reproduce (Opus direct)
  on the covered fraction" — `serve`/`payoff` with a recipe in the middle.

## Honest floor

The transfer fraction asymptotes at the judgment residual unless the chore is
fully recipe-able to code. Some part won't move down no matter how good the
recipe. The `author-fn` negative control is exactly the per-chore probe of how
far the ladder goes before the gate stops discriminating — a passing-but-
undiscriminating gate is the silent-rot failure mode, and it's worst at the
fuzzy (knowledge-work) end.

## The metric / next harness

For one real recurring chore, sweep recipe-rung × executor-tier. Report the
transfer fraction at each. The frontier = the cheapest / most-local / most-open
executor that still PASSES the gate at each rung. That is the apportionment-shift
demo's TRUE x-axis: the fraction of execution a weaker model takes via a
sharpening recipe — not "answers cached over a session."

Measured first cut (`crystal transfer`, entity→struct, n=7, suggestive): none
57% · plan 43% · recipe 71% · pseudocode 71% · code 100% · Opus-direct 86%.
Non-monotonic — the vague `plan` rung scored WORSE than no recipe (a bad
abstraction level misleads a weak model more than silence); the code rung beat
Opus-direct (the generator is the golden by construction). Each rung needs its own
verifier: code → `defn test`; plan → **plancheck** (the plan rung measured worst
*because* it was ungated).

## The serve layer (the library)

A crystallized artifact does not serve itself — it is served from a library behind
a gate. `internal/library` (`crystal library`) is that tier, modeled on
groupchat's deployed meme system: entries carry `deploy_when` / `too_much_if` /
`mechanism` / match-tokens / `min_conf`, are matched per-context behind a
CONFIDENCE + COOLDOWN gate, and ABSTAIN when uncertain — a wrong artifact is worse
than no artifact, which is "no verifier, no crystallization" at the serve layer.
The serve decision is deterministic (no model), so the serve tier itself is cheap;
the artifact it hands back is the recipe a weaker executor then runs. groupchat is
the existence proof this works on fuzzy, non-code work (see
`docs/STAGE_INTERFACES.md`).
