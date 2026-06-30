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

## Siblings instantiate the stages — three reuse modes

The stages above are satisfied by existing sibling projects (all the author's),
in one of three modes. Pick by who has to change:

- **Lift** — copy the logic into crystal. For small, self-contained, elegant
  primitives crystal should *own* with no runtime dependency.
- **Depend** — shell out / import and use as-is at runtime. For stable interfaces
  the sibling already exposes. Worked example: `internal/defnverify` swapped its
  fragile text-scraper for `defn impact --json` — "depend on the better interface
  that already exists," no lift, no dispatch. It also handed over `blast_radius`
  (a categorical Selector signal) and `source_file` (so `sediment.Locate` reads
  one file, no dir scan).
- **Dispatch** — message the sibling's live session for a feature/bug it lacks.
  Only when the *sibling itself* must change (e.g. a `defn test <name>` CLI so the
  Verifier is pure-defn). Use sparingly — it adds work to a project that may be
  under its own deadline.

Stage → sibling: discover = calque (code copies) / costean (transcript *moves* —
but that's a personality mirror, wrong target) / **slimemold** (claim topology:
load-bearing-but-unverified = the knowledge-work gate signal); select = adit
(relocatable / blast-radius); parse+verify+apply = defn; **serve = groupchat**.

## The serve tier is groupchat-modeled (the library)

`internal/library` (`crystal library`) is the serve layer of the recipe ladder —
lifted from groupchat's deployed meme system. A library of crystallized artifacts,
each with metadata (`deploy_when` / `too_much_if` / `mechanism` / `key` /
match-tokens / `min_conf`), is matched per-context behind a CONFIDENCE + COOLDOWN
gate that ABSTAINS when uncertain. The serve decision is deterministic (no model)
— that's what makes it the cheap tier. Cooldown raises the bar to re-fire the same
entry within a window (a strong-enough match overrides it); demote-on-drift
removes an entry until re-promoted.

The deeper point: **groupchat is the existence proof that crystal's whole loop
already works on fuzzy, non-code work.** slimemold (claim topology = the gate
signal), crowdwork ("the material is already there" = discovery), groupchat
(serve-from-library + confidence/cooldown + abstain-over-wrong) and lucida
(passive minting) are all the same *watch → detect → serve* ambient loop the
README cites. Every meme drop is a shift-left: a matched library artifact serves
instead of the frontier generating fresh humor, gated by fit + cooldown. The
metadata maps 1:1 to crystal's gate discipline; "wrong meme is worse than no meme"
is "no verifier, no crystallization" at the serve layer.

### The serve tier now has an Authorer (watch→author closed)

The library was hand-seeded; `crystal sweep --emit-library` closes the
watch→author half — the serve-layer twin of `--emit-dispatch`. A discovered
constraint (re-encoded across N projects, deterministic) is authored by Opus into
a candidate `library.Entry` (recipe artifact + `match`/`avoid`/`deploy_when`
metadata), then gated by `library.GateEntry` (deterministic, no serve): the entry
must SERVE on its realistic positives, ABSTAIN via the too-much guard where it
should hold back, and NEVER serve on a standing benign-context set. A passing
entry is PROPOSED to `.crystal-proposals/`, never auto-added.

The gate is not ceremonial — on the first live run it rejected three *distinct*
real authoring defects (description-positives the keyword matcher can't fire on;
discovery evidence folded in as positives, embedding the remediation language an
Avoid token guards on; generic-scope `avoid` tokens that suppress the entry's own
positives) before passing. The gated, model-authored decision is the *triggering*;
the recipe *body* is the fuzzy residual the serve-time abstain-over-wrong
discipline bounds. See `docs/NEXT.md`.
