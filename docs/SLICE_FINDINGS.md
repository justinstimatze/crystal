# The v1 slice — map-reduce + verifier on a real chore (2026-05-29)

`crystal triage` is the first thing that *ships* rather than measures: the whole shift-left stack
end to end on a real chore — categorize your actual Claude Code Bash usage — over the real redacted
command corpus. It's the artifact the eight experiments were pointing at.

## The stack (no frontier model is ever called)

- **verifier (deterministic rules)** — leading-binary rules cover the easy fraction for free *and*
  gate the cheap model: where a rule fires and the model disagrees, the rule wins and the divergence
  is flagged. Producer-verifier asymmetry, made concrete.
- **map (cheap model, Haiku)** — classifies only the *uncovered residual* the rules miss.
- **reduce (deterministic)** — tallies by category. A model is never asked to count (the `aggregate`
  lesson, applied).

## Result on the real corpus (48 unique Bash commands)

```
verifier (deterministic):  37/48 = g 0.77   (free, no model)
map (cheap model):         11/48 = 0.23     (the residual)
gate caught:               9 cheap-model divergences (rule fired, model disagreed → rule won)
frontier model calls:      0
```

Usage breakdown (the actual chore output): search/inspect 13, git 10, build/test 8, network 7,
file-edit 6, other 2, install 1, nav 1.

## "Dirty hands make us right" — shipping on real data found a verifier bug

The first run scored g 0.71 with **nav 6** — and the `--verbose` rows showed why: real commands are
**compound** (`cd X && git add && git commit`, `cd dir && rg ...`), and a naive leading-token rule
classified them by the leading `cd` → mislabeled the command's *purpose* as navigation. Worse, the
verifier then **overrode the cheap model's more-correct answer** (`git`/`search`) ~4 times — the
gate only has teeth when the rule is actually right, and a leading-token rule isn't, on compounds.

No synthetic experiment surfaced this; real data did. Fix: scan the `&&`/`;` segments and let the
first real *action* win over a leading `cd`. After the fix: g 0.71→**0.77**, nav 6→**1**, the
compound commands reclassified correctly (git 8→10, search 11→13), and the gate's remaining 9
divergences are the legitimate kind (rule right, model wrong: `mkdir`→file-edit, `chmod`→file-edit,
`ls`→inspect). This is the thesis demonstrated in the *development process itself*: a cheap
deterministic rule has its own real-world error mode, and "the verifier wins" is only safe once the
verifier is actually right.

## What this proves (and doesn't)

- **Proves the stack composes end to end on real data**: 77% of the chore done by deterministic
  rules at zero model cost, the cheap model touching only the 23% residual, deterministic reduce, no
  frontier call — and a useful real output. The value-prop is no longer only a hypothesis; one real
  chore runs the full deterministic-default + cheap-residual + verifier-gate pipeline.
- **Doesn't prove** the served-hook/latency-payoff end (this is a one-shot batch over a corpus, not a
  live PreToolUse hook serving in place of a frontier call), nor crystallize/demote-on-drift (the
  rules are hand-written, not authored by a tier and re-authored on drift). Those remain the roadmap.
- The verifier rules are hand-authored here; the "expensive tier authors the rules, re-authors on
  drift" loop (the actual crystal mechanism) is the next step — `triage` is the *target* a
  crystallized chore should compile to, built by hand once to prove the shape.

## Bottom line

The slice ships: a real chore done by the full shift-left stack, zero frontier calls, g=0.77, the
cheap model only on the residual, deterministic counting — and building it on real data corrected
the deterministic verifier in a way the synthetic experiments couldn't. Dirty hands.
