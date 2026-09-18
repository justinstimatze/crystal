package cmd

import (
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/library"
)

// LibraryCmd drives crystal's serve-from-a-library tier over a stream of
// contexts — the way groupchat fires memes over a live session. A library of
// crystallized artifacts is matched per-context behind a CONFIDENCE + COOLDOWN
// gate; it serves the best match when it is confident and not over-firing, and
// ABSTAINS otherwise (a wrong artifact is worse than no artifact). The serve
// decision is deterministic (no model) — the cheap tier.
//
// The demo stream exercises every behavior: serve on a strong match, abstain on
// cooldown when the same entry fires twice in a row, abstain on a weak match,
// the too-much guard, and demote-on-drift (a demoted entry never serves).
type LibraryCmd struct {
	Lib     string `help:"Library spec (crystallized artifacts + groupchat-style metadata)." default:"testdata/library/entries.json"`
	Verbose bool   `help:"Print each entry's confidence and the served artifact body."`
}

func (c *LibraryCmd) Run() error {
	lib, err := library.Load(c.Lib)
	if err != nil {
		return usageError{fmt.Errorf("load library %s: %w", c.Lib, err)}
	}

	fmt.Printf("library: serve crystallized artifacts from a library behind a confidence + cooldown gate\n")
	fmt.Printf("  %d entries across rungs: %s\n\n", len(lib.Entries), rungSummary(lib))

	// A demo context stream (stands in for a live session's requests).
	stream := []string{
		"please git add -A and commit",                           // → guard fires
		"git add -A again right now",                             // → same entry on cooldown → abstain
		"generate a Go struct type for this entity with fields",  // → entity-to-struct
		"classify this bash shell command into a category",       // → categorize
		"git add explicit file main.go",                          // → guard matches but 'explicit' trips too-much
		"verify this quote against its source citation verbatim", // → quote-verify
		"just a paraphrase summary of the source",                // → quote-verify matches but 'paraphrase' trips too-much
		"do some git add stuff",                                  // → weak guard match → abstain-low-conf
	}

	fmt.Println("=== serving the context stream ===")
	served, abstained := 0, 0
	for step, ctx := range stream {
		d := lib.Serve(ctx, step)
		name := "—"
		if d.Entry != nil {
			name = d.Entry.Name
		}
		fmt.Printf("  [%d] %-9s %-20s conf %.2f%s  %q\n", step, mark(d), name, d.Confidence, thrNote(d), truncate(ctx, 42))
		if c.Verbose && d.Served() {
			fmt.Printf("        serve: %s\n", d.Entry.Artifact)
		}
		if d.Served() {
			served++
		} else {
			abstained++
		}
	}

	// Demote-on-drift: the guard drifted; it must stop serving.
	fmt.Printf("\n=== demote-on-drift: 'guard-git-add-all' drifted → demote ===\n")
	lib.Demote("guard-git-add-all")
	d := lib.Serve("git add -A please", 99)
	fmt.Printf("  after demote, 'git add -A' → %s %s (a demoted artifact never serves)\n", mark(d), entryName(d))

	fmt.Printf("\n=== outcome ===\n")
	fmt.Printf("  served %d · abstained %d over %d contexts — precision over recall: a wrong artifact is\n", served, abstained, len(stream))
	fmt.Printf("  worse than no artifact, so the gate abstains on low confidence, cooldown, and too-much.\n")
	fmt.Printf("  The serve decision is deterministic (0 model calls); the matched artifact is the recipe a\n")
	fmt.Printf("  weaker executor then runs. This is groupchat's serve-from-library shape, generalized.\n")
	return nil
}

func mark(d library.Decision) string {
	switch d.Outcome {
	case "served":
		return "SERVE  "
	case "abstain-cooldown":
		return "cooldown"
	case "abstain-low-conf":
		return "low-conf"
	case "abstain-too-much":
		return "too-much"
	case "demoted-skip":
		return "demoted"
	default:
		return "no-match"
	}
}

func thrNote(d library.Decision) string {
	if d.Threshold > 0 && !d.Served() {
		return fmt.Sprintf(" (needed %.2f)", d.Threshold)
	}
	return ""
}

func entryName(d library.Decision) string {
	if d.Entry == nil {
		return "(no match)"
	}
	return d.Entry.Name
}

func rungSummary(lib *library.Library) string {
	by := lib.ByRung()
	var parts []string
	for _, r := range lib.Rungs() {
		parts = append(parts, fmt.Sprintf("%d %s", by[r], r))
	}
	return strings.Join(parts, ", ")
}
