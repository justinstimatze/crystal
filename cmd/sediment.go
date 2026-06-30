package cmd

import (
	"fmt"

	"github.com/justinstimatze/crystal/internal/defnverify"
)

// SedimentCmd runs the defn-backed Verifier stage over a repo's OWN arbitrary Go
// definitions — the step that turns "arbitrary Go logic sediments down the
// staircase" from proven-in-miniature (kong scaffolds) into a running claim.
//
// It uses defn for what the hand-rolled kong verifier could not give domain-
// generally: per-definition test COVERAGE (the Confidence gate) and scoped
// affected-test execution (the operational gate). The headline is the
// cheaply-verifiable FRACTION — the master variable from docs/THESIS.md — now a
// real number over real functions: the test-covered definitions are the ones a
// cheap replacement CAN be gated for (sediment-ready); the rest must stay on the
// frontier because crystal cannot verify them (no-verifier-no-crystallization).
//
// Requires the repo indexed: `defn ingest .` (writes a gitignored .defn/).
type SedimentCmd struct {
	Covered   string `help:"A test-covered definition to demo the gate PROMOTING (verifiable + green)." default:"Scan"`
	Untested  string `help:"An untested definition to demo the gate REFUSING (unverifiable)." default:"Commandish"`
	RunTests  bool   `help:"Actually run the covering tests for the --covered demo (slower; otherwise just report coverage)." default:"true"`
}

func (c *SedimentCmd) Run() error {
	if err := defnverify.Available(); err != nil {
		return usageError{fmt.Errorf("defn not available: %w", err)}
	}
	exported, err := defnverify.ExportedUnits()
	if err != nil {
		return usageError{fmt.Errorf("defn query (exported units): %w", err)}
	}
	untested, err := defnverify.Untested()
	if err != nil {
		return usageError{fmt.Errorf("defn untested: %w", err)}
	}
	verifiable := exported - untested

	fmt.Printf("sediment: the defn-backed Verifier stage over this repo's own arbitrary Go logic\n\n")

	fmt.Println("=== the cheaply-verifiable fraction (the master variable, measured) ===")
	fmt.Printf("  crystallizable surface (exported, non-test defs): %d\n", exported)
	fmt.Printf("  test-covered:       %d  ← a cheap replacement CAN be gated (defn runs the covering tests)\n", verifiable)
	fmt.Printf("  uncovered:          %d  ← UNVERIFIABLE — must stay on the frontier (refuse to crystallize)\n", untested)
	if exported > 0 {
		fmt.Printf("  sediment-ready:     %.0f%% of the exported surface is behind an operational verifier defn can run\n", 100*float64(verifiable)/float64(exported))
	}
	fmt.Println("  This is shift-left's real ceiling on this codebase: you place work as cheaply as you can")
	fmt.Println("  VERIFY it, and defn's coverage tells you exactly which fraction that is.")
	fmt.Println()

	fmt.Println("=== the gate, demonstrated on real definitions ===")

	// PROMOTE case: a test-covered definition.
	vp, err := defnverify.Verify(c.Covered)
	if err != nil {
		return usageError{fmt.Errorf("verify %q: %w", c.Covered, err)}
	}
	if !c.RunTests && vp.Verifiable {
		fmt.Printf("  %-14s Confidence=%d (covered) → VERIFIABLE; sediment-ready (tests not run, --run-tests=false)\n", vp.Name, vp.Confidence)
	} else {
		mark := "REFUSE"
		if vp.Verifiable && vp.TestsPass {
			mark = "PROMOTE-eligible"
		} else if vp.Verifiable {
			mark = "REJECT (tests red)"
		}
		fmt.Printf("  %-14s Confidence=%d → %s\n      %s\n", vp.Name, vp.Confidence, mark, vp.Detail)
	}

	// REFUSE case: an untested definition.
	vu, err := defnverify.Verify(c.Untested)
	if err != nil {
		return usageError{fmt.Errorf("verify %q: %w", c.Untested, err)}
	}
	fmt.Printf("  %-14s Confidence=%d → REFUSE\n      %s\n", vu.Name, vu.Confidence, vu.Detail)

	fmt.Printf("\nDogfood result: defn made the Verifier domain-general. The kong harness gated SCAFFOLDS by\n")
	fmt.Printf("build+probe; this gates ANY definition by its real tests + coverage, and honestly refuses the\n")
	fmt.Printf("%d uncovered ones. The author/serve loop (generating the cheap replacement) is the remaining\n", untested)
	fmt.Printf("piece; the VERIFIER + CONFIDENCE stage now runs on arbitrary Go logic. (Go-only; generalize later.)\n")
	return nil
}
