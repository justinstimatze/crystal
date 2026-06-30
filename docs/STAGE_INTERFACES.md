# Stage interfaces — crystal as a swappable-instance engine

Status: **design + a proof-of-one-swap.** The codegen loop (`discover → author →
gate → serve → demote`) is built kong-only today (`cmdspec` + `cmdverify` +
`discover`). This is the abstraction that turns that hand-rolled harness into
*instance #1* of a general engine whose stages are each satisfied by an existing
sibling project — the route from "one Go shape" to "arbitrary Go logic sediments
down the staircase."

## The five stages

```go
// A Unit is one crystallizable work-item: a typed, executor-portable schema
// plus a stable identity. (kong: a cmdspec.CmdSpec. general: a defn definition.)
type Unit interface {
    ID() string
    Schema() any
}

// Discoverer finds the recurring chore — the watch-don't-ask front half.
//   kong now: discover.Scan over cmdspec.ParseAll   (code-structural)
//   general:  calque (code structural-copies) | costean (transcript moves)
type Discoverer interface {
    Discover(corpus string) ([]Recurrence, error) // shape + recurrence + coverage
}

// Selector decides which recurrences precipitate cleanly (low coupling).
//   kong now: all command-shaped units
//   general:  adit (relocatable / blast-radius / coupling → a crystallizability score)
type Selector interface {
    Crystallizable(Recurrence) (ok bool, score float64, why string)
}

// Authorer is the frontier seam — writes the cheap-tier generator from examples.
//   stays crystal: Opus. No sibling replaces this.
type Authorer interface {
    Author(examples []Unit) (Generator, error)
    ReAuthor(examples []Unit, drifted Unit) (Generator, error)
}

// Verifier is THE load-bearing gate. This interface is where "arbitrary logic"
// becomes tractable: swap the instance, keep the loop.
//   kong now: cmdverify (go build + kong register + behavioral contract)
//   general:  defn affected-tests-pass + coverage  (domain-general)
type Verifier interface {
    Verify(produced string, u Unit) Verdict // {builds, registers, contract, ...}
    Confidence(u Unit) float64              // coverage — the gate's own honesty
}

// Applier lands a passing artifact — never auto-merge; propose.
//   kong now: writeProposal(.go.txt)
//   general:  defn edit/rename/move (auto-rebuild, all refs) — still proposal-gated
type Applier interface {
    Propose(artifact Generator, u Unit) (path string, err error)
}
```

The orchestrator — the loop already in `cmd/dogfood.go` — depends only on these.

| stage | instance #1 (built) | general instance (target) |
|---|---|---|
| Discoverer | `cmdspec` + `discover.Scan` | `calque` / `costean` |
| Selector | all-commands | `adit` relocatability |
| Authorer | Opus template | Opus (unchanged) |
| **Verifier** | `cmdverify` build+kong | **`defn` affected-tests + coverage** |
| Applier | `.go.txt` proposal | `defn` edit/rename (proposal-gated) |

## Why the Verifier is the load-bearing swap

`dogfood` proved the verifier — not the model tier — is the bottleneck: build +
register are nearly vacuous; only a *behavioral* check catches the real miss
(`docs/DOGFOOD_HARNESS.md`). `cmdverify` only knows kong commands; a test-based
verifier knows *any* Go definition. So the Verifier swap is the whole "arbitrary
logic" claim — and the `Confidence`/coverage method is what lets crystal honestly
*refuse* to crystallize under-tested logic (the dogfood slice-leak, generalized
into a hard gate: under-tested ⇒ g<1 ⇒ REJECT, loud).

## The proof-of-one-swap (built)

Rather than build all five interfaces, prove the one that matters: implement the
**Verifier twice, keep everything else kong**, and show the loop runs unchanged
when the instance swaps — *and* that the stronger instance raises coverage.

- `--verifier build` — the original: `go build` + kong registration + a
  binary-probe contract. The slice class **leaks** (a `[]string` rendered as a
  scalar passes a flag-*presence* probe while being wrong).
- `--verifier test` — the defn-`test` *shape* implemented with raw `go test`:
  generates a `kong.New().Parse()` harness asserting the contract as a test
  (enum off-list ⇒ error; a repeatable flag ⇒ binds 2 values). The slice class is
  now **caught** — `len(field) != 2` fails when the field is a scalar string.

Same loop, same chore, one swapped instance: the test verifier closes the exact
`g<1` boundary the build verifier left open. That is the abstraction's payoff
made concrete — and `go test` is defn-`test` minus blast-radius scoping, so the
production step is wiring this instance to `defn` (the deferred dependency), not
new design.
