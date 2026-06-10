package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/redact"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// ExtractCmd builds the redacted Record corpus from local transcripts.
type ExtractCmd struct {
	Home            []string `help:"Home dirs to scan (transcripts at <home>/.claude/projects/*/*.jsonl). Repeatable." required:""`
	Out             string   `help:"Output corpus dir." default:"testdata/corpus"`
	PerTool         int      `help:"Records to keep per registered tool." default:"40"`
	PerErrors       int      `help:"Additional error-class records to keep per registered tool (for outcome-class tests). Best effort — error results are rare." default:"10"`
	IncludeProjects []string `help:"Default-deny allowlist of project tokens (the dir name after -home-<user>-Documents-). If set, ONLY transcripts from these projects are extracted — used to keep the committed corpus to public repos only. Empty = all projects." name:"include-project"`
}

// projectDirPrefix strips the encoded -home-<user>-[Documents-] lead from a
// Claude Code project dir basename, leaving the project token (e.g. "lucida",
// "town-winze"). Used by the include-project allowlist.
var projectDirPrefix = regexp.MustCompile(`^-home-[A-Za-z0-9]+-(Documents-)?`)

// projectToken returns the allowlist-matchable token for a transcript file:
// the project-dir basename with the encoded home prefix stripped.
func projectToken(jsonlPath string) string {
	return projectDirPrefix.ReplaceAllString(filepath.Base(filepath.Dir(jsonlPath)), "")
}

// Run walks transcripts, balances per-tool, redacts, verifies fail-loud,
// and writes the corpus.
func (c *ExtractCmd) Run() error {
	if c.PerTool <= 0 {
		return usageError{fmt.Errorf("--per-tool must be > 0")}
	}
	registered := map[string]bool{}
	for _, t := range compare.Tools() {
		registered[t] = true
	}

	allow := map[string]bool{}
	for _, p := range c.IncludeProjects {
		allow[p] = true
	}

	counts := map[string]int{}
	errCounts := map[string]int{}
	seen := map[string]bool{} // tool_use_id dedup across both passes
	var kept []record.Record
	var files []string
	for _, home := range c.Home {
		matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, matches...)
	}
	// Default-deny project allowlist: when --include-project is set, keep only
	// transcripts from those projects (the committed corpus must stay public).
	if len(allow) > 0 {
		var filtered []string
		for _, f := range files {
			if allow[projectToken(f)] {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}
	sort.Strings(files) // deterministic fixture selection

	totalDropped := 0
	for _, f := range files {
		if quotaFull(counts, registered, c.PerTool) {
			break
		}
		recs, dropped, err := transcript.WalkFile(f)
		if dropped > 0 {
			redact.Warnf("%s: dropped %d malformed/oversized record(s)", f, dropped)
			totalDropped += dropped
		}
		if err != nil {
			redact.Warnf("%s: %v (continuing)", f, err)
		}
		for _, r := range recs {
			if !registered[r.Tool] || counts[r.Tool] >= c.PerTool || seen[r.ToolUseID] {
				continue
			}
			counts[r.Tool]++
			seen[r.ToolUseID] = true
			kept = append(kept, r)
		}
	}

	// Second pass: enrich with error-class records (rare in real data) so
	// the outcome-class corruptors have real samples. Best effort.
	if c.PerErrors > 0 {
		for _, f := range files {
			recs, _, _ := transcript.WalkFile(f)
			for _, r := range recs {
				if !registered[r.Tool] || !r.Result.IsError || seen[r.ToolUseID] || errCounts[r.Tool] >= c.PerErrors {
					continue
				}
				errCounts[r.Tool]++
				seen[r.ToolUseID] = true
				kept = append(kept, r)
			}
		}
	}

	// Redact, then verify fail-loud. If any secret survives, abort the
	// entire write — never ship a leak.
	var survivors []string
	for i := range kept {
		redact.Record(&kept[i])
		if err := redact.Verify(&kept[i]); err != nil {
			survivors = append(survivors, err.Error())
		}
	}
	if len(survivors) > 0 {
		for _, s := range survivors {
			redact.Warnf("ABORT: %s", s)
		}
		return fmt.Errorf("redaction left %d secret(s); refusing to write corpus", len(survivors))
	}

	if err := corpus.Save(c.Out, kept); err != nil {
		return err
	}

	// Report per-tool counts and shortfalls loudly.
	tools := compare.Tools()
	sort.Strings(tools)
	fmt.Printf("extracted %d records to %s (scanned %d files, dropped %d)\n", len(kept), c.Out, len(files), totalDropped)
	for _, t := range tools {
		status := "ok"
		if counts[t] < c.PerTool {
			status = fmt.Sprintf("SHORTFALL (<%d)", c.PerTool)
		}
		fmt.Printf("  %-8s %3d (+%d err)  %s\n", t, counts[t], errCounts[t], status)
	}
	return nil
}

func quotaFull(counts map[string]int, registered map[string]bool, perTool int) bool {
	for t := range registered {
		if counts[t] < perTool {
			return false
		}
	}
	return true
}
