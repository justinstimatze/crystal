# Dogfood harness — crystal crystallizes crystal's own boilerplate

Status: **BUILT** (`crystal dogfood`; `cmd/dogfood.go` + `internal/cmdspec` +
`internal/cmdverify`). Runs live (Opus authors the generator) and `--offline`
(deterministic stub); the build/register/contract verifier is real in both.

**Measured result (31 commands in `cmd/`):** build+register are nearly *vacuous*
on kong tags — they pass an enum-stripped scaffold cleanly; only the behavioral
contract (an off-list value must be rejected) catches the dropped enum →
demote → re-author → recover. So the verifier *had* to be behavioral, which is
the framing's core claim, measured on real code. The enum drop is **fundamental**
(a struct tag absent from every example — missed by both the crude stub and
Opus); the slice "leak" is **generator-dependent** (the stub renders `[]string`
as a scalar and slips past a presence contract — the measured `g<1` boundary;
Opus renders `.Type` verbatim and closes it). The run also surfaced and fixed two
bugs in the parser/verifier themselves (unexported `XCmd` helpers; a kong
acronym-kebab mismatch). The original design follows.

This is the next pudding after `DEMO_HARNESS.md`:
take the apportionment-shift loop off the toy domain (12 synthetic struct units)
and point it at a **real, non-trivial, recurring codegen chore in this repo**,
run live, and report whether the verifier catches a **real** generalization
miss — one discovered by running, not seeded. The toy demo proved the loop
*closes*; this tests whether the framing *holds* where the generator isn't
trivially synthesizable and the golden isn't pre-known.

## The target (genuinely dogfood, genuinely recurring)

**Generate a new kong subcommand scaffold for crystal itself.** The repo has
**33** subcommand structs in `cmd/*.go`, every one the same shape:

```go
type XCmd struct { /* flag fields with help/default struct tags */ }
func (c *XCmd) Run() error { /* ... */ }
```

plus one registration line in the `CLI` struct in `cmd/root.go`
(`X XCmd `cmd:"" help:"..."``). That is real recurrence with real history —
the commands accreted one at a time — and it is the chore crystal's own author
performs by hand every time a verb is added. Crystallizing it is crystal eating
its own dog food: the expensive tier authors a generator for "a crystal
subcommand" from the 33 existing ones, then serves new scaffolds for free.

## The upgrade that makes this a real test: the verifier is a build, not a golden

The toy demo verified each unit by **golden-equality** (gofmt byte-match against
a pre-known answer). That only works when you already know the exact output —
fine for a fixture, useless for real code where naming and formatting are free
choices. The dogfood verifier is **operational and golden-free**: a generated
subcommand is correct iff it passes a *layered property gate*, each layer
extending coverage `g`:

1. **builds** — `go build ./...` in a scratch copy succeeds (syntax + types).
2. **registers** — kong discovers it: `crystal <newcmd> --help` exits 0 and the
   verb appears in `crystal --help` (the struct tag in `root.go` is wired).
3. **contracts** — a behavioral smoke test passes: for a generator-class with a
   known contract (e.g. a `--cache-dir` flag defaults to `.crystal-cache`, an
   `enum:` flag rejects an off-list value with exit 2), assert the observable
   behavior on a fixture invocation.

This is the valence guard made concrete: the Go compiler and kong *are* the
verifier — maximally operational, no prose judge, two parties (compiler + CLI)
that agree deterministically. It is a strictly stronger gate than the demo's
golden because it certifies a *property*, not a memorized string.

## The real miss (discovered, not seeded) — and the finding it produces

Author the generator from a **held-out split** of the 33 commands that are all
the *regular* shape (flag struct + `Run`), then serve the **irregular** ones the
generator was never shown:

- `enum:"a,b,c"` constrained flags (`cmd/hook_loop.go`, `cmd/local_probe.go`),
- repeatable `Home []string` slice flags (11 commands),
- commands with non-`Run` helper methods, stdin readers, or an http server
  (`cmd/viz.go`, `cmd/hook.go`, `cmd/serve.go`).

A generator generalized from plain flag-structs will mis-render at least one of
these. **The point is to measure where each verifier layer's coverage ends:**

- A scaffold that doesn't compile → caught at layer 1. (structure)
- A scaffold that compiles but isn't registered (wrong/missing `root.go` tag) →
  caught at layer 2. (discoverability)
- **A scaffold that builds and registers but is behaviorally wrong** — an
  `enum` rendered as a plain `string` so an off-list value is silently accepted,
  or a default flipped — **builds and registers cleanly and LEAKS past layers 1
  and 2.** Layer 3 catches it *only if a contract was written for that class*;
  otherwise it is the residual.

That leak is the finding, and it is the honest test of the whole framing: a
build/register gate covers *structure*, not *behavior*. The measured output is a
**coverage table** — for each irregular class, which verifier layer caught it
(or that none did) — i.e. the empirical `g<1` boundary for this chore. The
demote-on-drift then fires on the caught misses (re-author with the irregular
example in scope, re-gate, re-serve); the *uncaught* leak is reported as the
frontier-residual that no deterministic gate here can absorb. This is the run
that either earns "typed schema + operational verifier shifts work off the
frontier at held quality" or shows exactly where it breaks.

## The loop (same state machine as the demo, new verifier + new corpus)

```
NO_ARTIFACT — first K regular commands are the worked examples (live: Opus reads
              the real cmd/*.go source; offline: the existing source IS the
              example, no model).
AUTHORING   — the expensive tier authors a generator (Go text/template or a
              small DSL) for "a crystal subcommand + its root.go registration"
              from the worked examples; gated by building a holdout in a scratch
              module (layers 1–2 ≥ threshold).
SERVING     — generate the remaining commands' scaffolds, build+register them in
              a scratch copy (0 model calls) — served-det.
DEMOTE      — an irregular command fails a verifier layer → demote that one to
              the frontier, re-author with it in scope, re-gate, re-serve. An
              uncaught behavioral leak is logged to the residual, not demoted
              (the gate didn't see it — honesty over a false green).
```

## New code (minimal glue over the demo harness)

1. **`cmd/dogfood.go` (`DogfoodCmd`)** — the driver. Reuses `cmd/demo.go`'s loop
   shape; swaps the unit type and the verifier.
   Surface: `crystal dogfood --corpus cmd --first-k 10 --scratch <tmp> [--offline|--live]`.
2. **A subcommand-`Unit` loader** — parse the 33 existing `cmd/*.go` into
   `(name, flags[], shape-features)` specs by reading the `XCmd struct` + its
   `root.go` registration. (Go `go/parser`/`go/ast` over `cmd/`, not regex.)
3. **The build/register/contract verifier** (`internal/verify/` or inline) —
   writes the generated scaffold into a **scratch Go module** (a throwaway copy
   of the CLI skeleton), runs `go build`, invokes the built binary for
   `--help`/registration, and runs the per-class contract checks. Returns a
   layered verdict `{builds, registers, contract}` per unit.
4. **Reuse**: the loop, two-regime `--offline`/`--live`, flow + staircase emit,
   and summary renderer all come straight from `cmd/demo.go`; `internal/llm`,
   `internal/flow` unchanged.

## Safety / scope cuts (this generates code into a real repo)

- **Generated scaffolds are gated PROPOSALS, never auto-written to `cmd/` and
  never auto-committed.** They are emitted to a scratch module, built and probed
  there, and the passing ones are written to an `--out` proposal dir for a human
  to review and move in. crystal does not edit its own `cmd/`/`root.go` in place.
- **Read-only on the corpus.** The 33 existing commands are parsed, never
  rewritten.
- **One chore.** Subcommand scaffolds only; not comparators, not the viz HTML.
  Other recurring chores in the repo reuse the same loop with a different
  unit-parser and verifier — out of scope here.
- **No new mechanism.** If a step needs something the demo loop + a build gate
  can't express, that's the signal it's reaching past the minimal cut.

## Two-regime reproducibility (mirror demo + bench)

- **`--offline`:** the generator is a deterministic stub synthesized from the
  parsed corpus features; the whole loop (parse → author → build → register →
  demote/re-author) runs key-free in CI. The build/register verifier is real in
  both regimes — it never needs a model. The seeded-free miss still surfaces
  because it is a property of the corpus, not the author.
- **`--live`:** Opus authors the generator from the real `cmd/*.go` source;
  latencies and the authored artifact are real (disk-cached). This is the run
  that produces the citable coverage table.

## Why this is the right dogfood

It removes the two things that made the demo easy to agree with: the toy domain
and the pre-known golden. The corpus is the repo's own real history; the verifier
is the repo's own compiler. If the loop shifts crystal's subcommand-authoring
off the frontier while the build/register/contract gate holds quality — and the
coverage table honestly marks where it doesn't — that is the framing earning its
keep on real work. If the gate leaks a behavioral miss, that leak is the
measured `g<1` boundary, which is a result, not a failure.
