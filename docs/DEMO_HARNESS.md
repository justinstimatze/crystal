# Demo harness — the apportionment-shift demo (minimal, buildable spec)

Status: **spec, not built.** This is the concrete design for the ROADMAP
"Demo & onboarding — a compelling first run" item. It specifies the smallest
harness that makes crystal's core claim *visible*: over one long, chore-dense
session, the apportionment of work slides from "big expensive model, always"
to "mostly cheap / deterministic, by default" — while a real project actually
gets completed, and a verifier underneath keeps quality honest.

## What the demo must show (the four properties)

1. **A staircase, not a step.** Per-unit cost falls in stages: the first few
   units on Opus, then a crystallized artifact serves the rest cheap/free.
2. **Real completion.** At the end there is a working N-unit artifact — the
   project is *done*, not a toy trace.
3. **The gate catching a real miss.** A seeded non-conforming unit must demote
   back to Opus, re-author, and re-promote — not silently pass.
4. **A measured curve.** Units-by-tier + cost/latency saved vs always-Opus +
   breakeven — numbers, bound to `crystal viz` so the shift animates.

The property that makes a target work is **recurrence with a built-in
verifier**: the same shaped chore repeats N times and each repetition is
deterministically checkable. A one-off project has nothing to crystallize.

## Target domain (minimal + genuinely crystallizable + verifiable)

**Generate one code unit per entity in a schema list.** Each unit is a
template instantiation (e.g. a Go struct + CRUD handler + table-driven test
for an entity). The crystallizable artifact is a **deterministic template**
that the expensive tier authors from the first few worked examples; the
per-unit verifier is **golden-equality** over the canonicalised output
(gofmt / byte-exact). Recurrence = the entity list. Drift = an entity whose
shape the template mis-renders.

This domain is chosen because it maps almost one-to-one onto machinery crystal
**already has**: author-a-deterministic-artifact-from-worked-examples,
gate-on-holdout-fidelity, serve-the-rest, demote-on-drift, re-author,
re-promote. The harness is mostly *rewiring existing components onto a new
chore*, not new mechanism. (Generalising to OpenAPI-client or codemod targets
is a later phase; the loop is identical, only the unit renderer changes.)

## The loop (state machine)

Per unit, in order:

```
NO_ARTIFACT ──collect K passing worked examples on Opus──▶ AUTHORING
AUTHORING   ──expensive tier writes template, gated on holdout fidelity≥0.95──▶ SERVING
SERVING     ──render via template (0 model), verify vs golden──▶ served-det
SERVING     ──golden fails (seeded drift)──▶ DEMOTED(unit→Opus) ──▶ RE_AUTHORING ──▶ SERVING
```

- **NO_ARTIFACT / AUTHORING:** unit produced by Opus (fate `deferred-model`),
  verified against its golden; passing units accrue as worked examples. At K
  passing, the expensive tier authors the template and it is gated on a
  holdout of already-produced units via `eval.Run` (fidelity ≥
  `eval.PromoteThreshold`). Promote → SERVING; reject → stay deferred, collect
  more (a lowered bar is never the path to promotion — more data is).
- **SERVING:** unit rendered by the template (code tier, 0 model calls),
  verified against its golden (fate `served-det`). A drift unit fails the
  golden → demote *that unit* to Opus (fate `deferred-model`), flag for
  re-author; the re-authored template includes it and re-served units count as
  `reserved_det` (fate `served-now`).

This is exactly the closed loop `cmd/hook_loop.go` already runs for the
Bash-categorisation chore — generalised to a codegen chore with a golden
verifier instead of the agreement oracle.

## New code (the minimal glue)

1. **`cmd/demo.go` (`DemoCmd`)** — the unit-loop driver / state machine above.
   Surface: `crystal demo --spec testdata/demo/entities.json --first-k 5
   --out docs/viz/demo.json [--offline|--live]`.
2. **`Spec` / `Unit` loader** + a small example spec
   (`testdata/demo/entities.json`, ~12 units incl. 2 seeded drift):
   ```go
   type Unit struct {
       Name   string            `json:"name"`
       Params map[string]any    `json:"params"`
       Golden string            `json:"golden"` // per-unit verifier target
       Drift  bool              `json:"drift"`  // seeded irregular unit
   }
   type Spec struct { Domain string `json:"domain"`; Units []Unit `json:"units"` }
   ```
3. **A template `Artifact`** implementing the existing `artifact.Artifact`
   interface (`internal/artifact/artifact.go`) — `Produce(unit) → Output` so
   it drops straight into the existing gate; compared against `Unit.Golden`.
4. **A golden comparator** for the unit domain (or reuse exact-match over a
   canonicalised string), registered like the per-tool comparators in
   `internal/compare/`.

## Reuse map (what already exists — cite before reinventing)

| Need | Reuse | File |
|---|---|---|
| Opus authoring + optional Haiku rung, disk-cached | `internal/llm` | `internal/llm/client.go` |
| Per-unit golden gate (fidelity ≥ 0.95, promote/reject) | `internal/eval` + `internal/compare` | `internal/eval/eval.go` |
| Author-from-examples + holdout gate | pattern from `AuthorCmd` | `cmd/author.go` |
| Serve det + latency / determinism measurement | pattern from `ServeCmd` | `cmd/serve.go` |
| Demote → re-author → re-promote (closed loop) | pattern from `HookLoopCmd` | `cmd/hook_loop.go` |
| Per-unit fate logging + viz binding | `internal/flow` | `internal/flow/flow.go` |
| Breakeven / amortisation for the summary | pattern from `AmortizeCmd` | `cmd/amortize.go` |
| Live dashboard render | `crystal viz` | `cmd/viz.go`, `docs/viz/` |

## Output (bound to the existing viz)

- **The completed artifact on disk** — N rendered units that all pass golden
  (the project is built).
- **`docs/viz/demo.json`** = one `flow.Record` (Sankey: `stream → served-det`,
  `stream → deferred-model`, `reauthor → served-now`) + a per-unit
  `flow.HistoryEntry` JSONL (the staircase time series). `crystal viz` already
  binds to exactly these shapes — no renderer change needed.
- **A summary table** (reuse the `crystal bench` renderer): units by fate
  (Opus / authored-det / re-served), median latency and $ per tier, blended
  saving vs always-Opus, and breakeven unit count.

## Two-regime reproducibility (mirror `crystal bench`)

- **`--offline`:** the template author is a deterministic stub (fixed,
  key-free) so the *whole staircase runs in CI and screenshots* without a
  credential — the demo is reproducible by anyone, like the bench OFFLINE
  block. The seeded-drift demote/re-author still fires (it is mechanical, not
  model-dependent).
- **`--live`:** Opus actually authors the template from worked examples and
  serves the Opus baseline; numbers become real (disk-cached, so re-runs are
  free). Requires `ANTHROPIC_API_KEY`.

## Scope cuts (what the minimal version is NOT)

- **Not a live Claude Code session** driving real `Edit`/`Write` tool calls.
  It is a self-contained driver that *stands in* for one — the fate signal is
  identical (each unit is a "request" with a tier + gate outcome). Hooking a
  real session (PreToolUse capture → `internal/flow`) is a later phase; this
  spec deliberately avoids it so the demo is deterministic and runnable in CI.
- **One domain only** (entity → code). OpenAPI-client and codemod targets
  reuse the same loop with a different unit renderer; out of scope here.
- **No new mechanism.** If a step needs mechanism crystal doesn't already
  have, it is a red flag that the demo is reaching past the minimal cut.

## Why this is the right first demo

It is the project's thesis made visceral — Opus writes the generator, then the
generator runs for free behind a verifier — which is also the tool-making
lineage in `docs/PRIOR_ART.md` (LATM / DreamCoder / ReGAL) in motion. And it
is the one thing the platform's *static* apportionment cannot do (per the
`model:` / `effort:` frontmatter and the hardcoded Explore→Haiku route):
synthesise a verified deterministic replacement for a recurring chore and
demote it on drift.
```
