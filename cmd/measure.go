package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/measure"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// MeasureCmd sweeps signature granularities over the full local substrate
// to surface whether a crystallizable pattern (frequent AND deterministic)
// exists. Read-only: it never writes a corpus, so no redaction is needed.
type MeasureCmd struct {
	Home       []string `help:"Home dirs to scan (transcripts at <home>/.claude/projects/*/*.jsonl). Repeatable." required:""`
	MinSamples int      `help:"Frequency floor for a crystallization candidate." default:"30"`
	Promote    float64  `help:"Determinism floor (largest output class / group size)." default:"0.95"`
	JSON       bool     `help:"Emit the sweep as JSON."`
}

func (c *MeasureCmd) Run() error {
	registered := map[string]bool{}
	for _, t := range compare.Tools() {
		registered[t] = true
	}
	var files []string
	for _, home := range c.Home {
		m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, m...)
	}
	sort.Strings(files)

	acc := measure.New()
	byTool := map[string]int{}
	total := 0
	for _, f := range files {
		rs, _, _ := transcript.WalkFile(f)
		for _, r := range rs {
			if !registered[r.Tool] {
				continue
			}
			acc.Add(r)
			byTool[r.Tool]++
			total++
		}
	}
	if total == 0 {
		return usageError{fmt.Errorf("no records found under given homes")}
	}

	reports := acc.Report(c.MinSamples, c.Promote)
	if c.JSON {
		b, _ := json.MarshalIndent(reports, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	tools := compare.Tools()
	sort.Strings(tools)
	fmt.Printf("scanned %d files, %d registered-tool records\n", len(files), total)
	for _, t := range tools {
		fmt.Printf("  %-8s %d\n", t, byTool[t])
	}
	fmt.Printf("\ncrystallizable = group with N>=%d AND determinism>=%.2f\n\n", c.MinSamples, c.Promote)
	for _, rep := range reports {
		fmt.Printf("granularity %-11s groups=%-5d  N>=%d: %-3d  crystallizable: %d\n",
			rep.Name, rep.TotalGroups, c.MinSamples, rep.GroupsAtSize, len(rep.Crystallizable))
		for _, g := range rep.Crystallizable {
			fmt.Printf("    ★ %-40s N=%-4d det=%.2f\n", trunc(g.Sig, 40), g.N, g.Determinism)
		}
		// Show the largest groups too, so a near-miss (frequent but not
		// deterministic — the gh-api trap) is visible, not hidden.
		for _, g := range rep.TopBySize {
			if g.Crystallizable(c.MinSamples, c.Promote) {
				continue
			}
			fmt.Printf("      %-40s N=%-4d det=%.2f distinct=%d\n", trunc(g.Sig, 40), g.N, g.Determinism, g.DistinctOutputs)
		}
		fmt.Println()
	}
	return nil
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}
