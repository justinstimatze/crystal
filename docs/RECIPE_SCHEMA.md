# The recipe schema — a crystallized artifact as a typed object

Status: built. `internal/recipe` + `crystal recipe`. The artifact-schema half of
the recipe ladder (`docs/RECIPE_LADDER.md`).

## Why a schema at all

crystal's whole value prop is that a crystallized artifact is *portable across
executor tiers* — authored by a strong model, run by a weaker one (or by no model
at all). That is only true if the portable unit has a **shape**. A recipe stored
as a prose blob that a strong model emits and a weak one re-parses ad hoc isn't
portable; it's just another prompt. So the artifact is a typed, diffable object
(`recipe.Recipe`), and it can be linted like code.

## The design, after Odendahl

The schema follows Manuel Odendahl's ("wesen", go-go-golems) framing in
[*Tool use and notation as shaping LLM generalization*](https://the.scapegoat.dev/tool-use-and-notation-as-generalization-shaping/)
(25 Feb 2026). His thesis is crystal's, stated more sharply than we had: notation,
tools, and code don't make the model smarter — in his words, they "make the task
simpler." A recipe is one unit of that move, which he names **generalization
shaping**: re-representing an out-of-distribution chore as a sequence of
in-distribution steps so the executor's *residual task* is easy, while correctness
is carried by deterministic machinery on the other side.

The schema encodes the two axes he tracks separately, plus the two things that
make an interface easy to target:

| schema field | Odendahl's concept |
|---|---|
| `rung` | **actuator expressivity** — how powerful the notation the executor targets (prose plan → recipe → pseudocode → code) |
| `executor` | the **model-burden floor** — the weakest tier that can still run the body. "The sweet spot ... is maximizing actuator expressivity while minimizing model burden." |
| `inputs` / `output` | a **narrow-waist interface** — typed, few degrees of freedom |
| `verifier` | the deterministic machinery that **carries correctness** off the model |
| `residual` | "what's left for the model after all that shaping" |

### Narrow waist → a degrees-of-freedom metric

Odendahl: "The model's search problem scales with the degrees of freedom at the
interface, so minimize them" — a tool taking `{category, date_range}` is easier to
target than one taking `{query: any}`. `Recipe.DegreesOfFreedom()` makes that
countable: input count plus a penalty per loosely-typed (`any`/`json`/`map`)
field. The broken example recipe scores **5** on a single `{query: any}` input;
the real entity→struct recipes score **2** on `{name, fields}`.

### The lint enforces the rung↔executor↔verifier coupling

The load-bearing checks in `Recipe.Validate()` encode that the rung *determines*
the burden floor:

- a **code** or **memory** rung must run with `executor: none` — a deterministic
  rung that still needs a model is a contradiction;
- a **code** rung must be gated by a *deterministic* verifier (a model-judge can't
  carry correctness for a no-model rung);
- a **plan/recipe/pseudocode** rung is run by a model, so `executor: none` is
  invalid;
- a **plan** rung gated by `verifier: none` is flagged as ungated — a plan is
  gated by **plancheck** (the seam to the gated-plan transfer harness, #3).

`crystal recipe` runs this over a directory and renders the result as a **ladder
per intent** (highest model burden first), exiting non-zero if any recipe fails —
so the schema is CI-gateable.

## Structured output, not printf (glazed)

`crystal recipe` emits structured rows (a `lintRow` per recipe; `--json` dumps
them verbatim) and renders the table *from* those rows. This is deliberate: it
practices Odendahl's glazed data philosophy from
[*I want my software to be visionary*](https://the.scapegoat.dev/i-want-my-software-to-be-visionary-the-go-go-golems-ecosystem/)
(12 Mar 2023) — applications hold a rich representation of their data, yet "most
programs then use some variant of printf to export that data," forcing the reader
to re-infer the structure that was just thrown away. The lint rows compose into the
next stage instead of being terminal. (The same cleanup is owed to `transfer`,
`library`, and `sediment`, which still printf.)

## Honest relationship to Odendahl's work

He originated the vocabulary *and* builds the component pieces this schema names —
the actuators (`go-go-goja` is his essay's Rung-5 `eval()` "programmable actuator"
as a project), the verifiers/eval harnesses (`rag-evaluation-system`), the typed
structured-data substrate (`glazed`), the prompt-as-command layer (`pinocchio` /
`geppetto`). What his public work does **not** do is the *automated sedimentation
loop* — discover a recurring chore, author a cheaper artifact for it, gate it, and
**demote it on drift** — that moves a chore *down* these rungs without a human in
the seat. He assembles the rungs by hand into workflows; crystal's contribution is
automating the descent across them, behind a verifier, with demotion. Two
practitioners on the same lineage at different layers (see
`hybrid/skills/hybrid-loops/references/PRIOR_ART.md`). The schema here is crystal
adopting his axes so the automated loop has a typed thing to move.
