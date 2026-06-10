package cmd

import "fmt"

// sweep_stull.go is the crystal→stull seam: turn a discovered CONSTRAINT into a
// provably-sound stull machine. stull (a sibling DSL) statically PROVES the
// emitted control flow is sound — reachable halt, no orphan states, fuel-bounded,
// and crucially NO UNGATED ORACLE on the control path. That formal proof is the
// upgrade over --emit-dispatch's empirical gate ("matched the bad forms, missed
// the benign set"): sound by construction, not by sampling.
//
// The seam is DEFERRED for the public release. The backend it imports
// (github.com/justinstimatze/stull) is not yet public, and for the stateless
// block-this-command constraints crystal surfaces today the emitted machine is
// trivial (2 states, no oracle) — so the formal proof is near-vacuous and
// --emit-dispatch already serves the same class standalone. stull earns its keep
// only when crystal crystallizes STATEFUL control: a fenced oracle that must be
// gated, demote-on-drift transitions, fuel-bounded loops. That's the vision, not
// the present, so the build dependency is dropped until crystal reaches it. The
// architectural intent lives on in docs/ROADMAP.md; re-attaching is one import.

// emitConstraintStull is the --emit-stull path. It is currently a stub: the stull
// formal-proof backend is not yet public, so the path explains the seam and points
// at the available crystal-native serve path (--emit-dispatch) instead of emitting.
func (c *SweepCmd) emitConstraintStull(signature string) error {
	fmt.Printf("crystal sweep --emit-stull: constraint %q\n\n", signature)
	fmt.Printf("  The stull formal-proof backend is not yet public, so this path is deferred.\n")
	fmt.Printf("  stull statically proves emitted control flow is sound (reachable halt, no\n")
	fmt.Printf("  orphan states, fuel-bounded, no ungated oracle) — the upgrade over the\n")
	fmt.Printf("  empirical gate. It earns its keep on STATEFUL crystallization (a gated\n")
	fmt.Printf("  oracle, demote-on-drift loops), which crystal's substrate does not yet\n")
	fmt.Printf("  surface. See docs/ROADMAP.md for the seam.\n\n")
	fmt.Printf("  Available today: `crystal sweep --emit-dispatch` authors + GATES a regex\n")
	fmt.Printf("  matcher for this constraint and emits a stateless block-every-time rule.\n")
	return nil
}
