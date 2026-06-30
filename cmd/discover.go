package cmd

import (
	"fmt"

	"github.com/justinstimatze/crystal/internal/cmdspec"
	"github.com/justinstimatze/crystal/internal/discover"
)

// DiscoverCmd is the watch-don't-ask front half of the codegen loop: it scans a
// code corpus, finds the DOMINANT recurring unit-shape by structure, and reports
// whether it clears the recurrence bar to be a crystallization candidate — the
// step that turns `dogfood`'s hand-pointed corpus into a found chore. It does not
// presume the "*Cmd" naming convention; the shape is detected structurally
// (help-tagged flag fields), so it finds a command not named *Cmd and rejects a
// bare-field helper that is.
//
// This is the codegen sibling of `sweep`/`measure` (which discover recurring
// Bash constraints/procedures). The recurrence + coverage it reports is the
// honest signal that a chore is worth crystallizing at all.
type DiscoverCmd struct {
	Corpus       string `help:"Directory of Go files to scan for a recurring command shape." default:"cmd"`
	MinRecurrence int   `help:"Recurrence floor: a shape must recur at least this many times to be a crystallization candidate." default:"5"`
	Verbose      bool   `help:"List the members of each discovered shape cluster."`
}

func (c *DiscoverCmd) Run() error {
	all, err := cmdspec.ParseAll(c.Corpus)
	if err != nil {
		return usageError{fmt.Errorf("scan corpus %s: %w", c.Corpus, err)}
	}
	rep := discover.Scan(all, c.MinRecurrence)

	fmt.Printf("discover: scanned %d struct(s) in %s/ for a recurring command shape (no naming convention assumed)\n\n", rep.Scanned, c.Corpus)

	fmt.Println("=== shape clusters (by recurrence) ===")
	for _, cl := range rep.Clusters {
		fmt.Printf("  %-40s recurrence %d\n", cl.Signature, cl.Recurrence())
		if c.Verbose {
			for _, m := range cl.Members {
				fmt.Printf("      %s\n", m.Name)
			}
		}
	}
	fmt.Println()

	dom := rep.Dominant
	reg, enum, slice := dom.Breakdown()
	fmt.Println("=== dominant shape (the crystallization candidate) ===")
	fmt.Printf("  shape:      %s\n", dom.Signature)
	fmt.Printf("  recurrence: %d  (coverage %.0f%% of scanned structs)\n", dom.Recurrence(), rep.Coverage*100)
	fmt.Printf("  variety:    %d regular · %d enum · %d slice  (the irregular ones are where a generator first drifts)\n", reg, enum, slice)
	fmt.Printf("  unit schema: {Name, Fields:[{Name, Type, Help, Default, Enum}]}\n\n")

	if rep.Candidate {
		fmt.Printf("Decision: CANDIDATE — the '%s' shape recurs %d ≥ %d times. This is a crystallizable chore;\n", dom.Signature, dom.Recurrence(), c.MinRecurrence)
		fmt.Printf("crystallize it with: crystal dogfood --corpus %s\n", c.Corpus)
		fmt.Println("(Discovery found the chore by structure — not by the '*Cmd' name; a help-tagged struct under")
		fmt.Println("any name would be found, and a bare-field helper named *Cmd would be rejected.)")
	} else {
		fmt.Printf("Decision: NO CANDIDATE — the dominant shape recurs only %d < %d times; nothing crystallizable here.\n", dom.Recurrence(), c.MinRecurrence)
		fmt.Println("(Loud by design: crystal crystallizes only what it finds RECURRING.)")
	}
	return nil
}
