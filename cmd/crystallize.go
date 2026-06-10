package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/crystallize"
	"github.com/justinstimatze/crystal/internal/eval"
	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/redact"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// CrystallizeCmd runs the full lifecycle on one pattern: discover the
// dominant exact-signature group under a match, propose a modal hook,
// promote-gate it, serve+drift-monitor the holdout, and emit a redacted
// deployable artifact.
type CrystallizeCmd struct {
	Home      []string `help:"Home dirs to scan. Repeatable." required:""`
	Match     string   `help:"Substring identifying the command family to crystallize." required:""`
	Tool      string   `help:"Tool to match." default:"Bash"`
	TrainFrac float64  `help:"Fraction of occurrences used to propose+promote." default:"0.4"`
	K         int      `help:"Demotion divergence count (M)." default:"3"`
	Window    int      `help:"Demotion sliding window (W)." default:"10"`
	Out       string   `help:"Dir to write the crystallized artifact." default:"crystallized"`
}

func (c *CrystallizeCmd) Run() error {
	if c.TrainFrac <= 0 || c.TrainFrac >= 1 {
		return usageError{fmt.Errorf("--train-frac must be in (0,1)")}
	}
	var files []string
	for _, home := range c.Home {
		m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, m...)
	}
	sort.Strings(files)

	// Collect matching records, grouped by EXACT signature — so we
	// crystallize the dominant deterministic command, not a blur of variants.
	groups := map[string][]record.Record{}
	for _, f := range files {
		rs, _, _ := transcript.WalkFile(f)
		for _, r := range rs {
			if r.Tool != c.Tool {
				continue
			}
			cmd, _ := r.Args["command"].(string)
			if c.Tool == "Bash" && !strings.Contains(cmd, c.Match) {
				continue
			}
			groups[eval.SignatureExact(r)] = append(groups[eval.SignatureExact(r)], r)
		}
	}
	if len(groups) == 0 {
		return usageError{fmt.Errorf("no %s records match %q", c.Tool, c.Match)}
	}

	// Pick the largest exact-signature group.
	var bestKey string
	for k, g := range groups {
		if bestKey == "" || len(g) > len(groups[bestKey]) {
			bestKey = k
		}
		_ = g
	}
	recs := groups[bestKey]
	sort.SliceStable(recs, func(a, b int) bool { return recs[a].Timestamp.Before(recs[b].Timestamp) })
	pattern := patternLabel(recs)

	spec := crystallize.Run(pattern, recs, c.TrainFrac, c.K, c.Window)

	// Narrate the lifecycle.
	fmt.Printf("CRYSTALLIZE  %s  [%s]\n", pattern, spec.Tool)
	fmt.Printf("  discover : %d total occurrences in dominant exact-signature group (of %d matched groups)\n", len(recs), len(groups))
	fmt.Printf("  propose  : modal hook, train N=%d, determinism=%.3f\n", spec.TrainN, spec.TrainDeterminism)
	fmt.Printf("  gate     : %s\n", strings.ToUpper(spec.PromoteDecision))
	if !spec.Promoted {
		redact.Warnf("REFUSED to crystallize %q (%s) — not deployed", pattern, spec.PromoteDecision)
		return nil
	}
	fmt.Printf("  serve    : holdout N=%d, servedCorrect=%d, leaked=%d, rule=%d-in-%d\n",
		spec.HoldoutN, spec.ServedCorrect, spec.Leaked, spec.DemotionRule.M, spec.DemotionRule.W)
	if spec.DemotedAtIndex >= 0 {
		fmt.Printf("  demote   : DEMOTED at holdout index %d (drift detected, served wrong %d times first)\n", spec.DemotedAtIndex, spec.Leaked)
	} else {
		fmt.Printf("  demote   : %s\n", strings.ToUpper(spec.ServeDecision))
	}

	// Redact the served output before persisting the artifact (sovereignty).
	redactSpecOutput(&spec)
	if err := writeArtifact(c.Out, spec); err != nil {
		return err
	}
	fmt.Printf("  artifact : written to %s/\n", c.Out)
	return nil
}

func patternLabel(recs []record.Record) string {
	if len(recs) == 0 {
		return "?"
	}
	if cmd, ok := recs[0].Args["command"].(string); ok {
		cmd = strings.SplitN(strings.TrimSpace(cmd), "\n", 2)[0]
		if len(cmd) > 80 {
			cmd = cmd[:80]
		}
		return cmd
	}
	return recs[0].Tool
}

func redactSpecOutput(spec *crystallize.Spec) {
	tmp := record.Record{Result: spec.ServedOutput}
	redact.Record(&tmp)
	spec.ServedOutput = tmp.Result
}

func writeArtifact(dir string, spec crystallize.Spec) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	name := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, spec.Pattern)
	if len(name) > 60 {
		name = name[:60]
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644)
}
