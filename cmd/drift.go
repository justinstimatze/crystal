package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/drift"
	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// DriftCmd runs a temporal-replay drift experiment over a real pattern:
// promote a modal hook on the pattern's earliest occurrences, then stream
// the rest in timestamp order and report whether/when drift demotes it and
// how many wrong outputs leaked first.
type DriftCmd struct {
	Home        []string `help:"Home dirs to scan. Repeatable." required:""`
	Match       []string `help:"Substring(s) to match a Bash command (one experiment per match)." required:""`
	Tool        string   `help:"Tool to match." default:"Bash"`
	TrainFrac   float64  `help:"Fraction of (time-ordered) occurrences used to promote the hook." default:"0.4"`
	K           int      `help:"Divergences (M) to demote." default:"3"`
	Window      int      `help:"Sliding window (W) the M divergences must fall within. W==K reproduces the brief's consecutive-K rule; W>K bounds leak under intermittent drift." default:"3"`
	Promote     float64  `help:"Promote determinism threshold." default:"0.95"`
	InjectShift bool     `help:"Controlled test: overwrite the back half of the stream with a single shifted output to verify demotion fires."`
	JSON        bool     `help:"Emit reports as JSON."`
}

func (c *DriftCmd) Run() error {
	if c.TrainFrac <= 0 || c.TrainFrac >= 1 {
		return usageError{fmt.Errorf("--train-frac must be in (0,1)")}
	}
	if c.Window < c.K {
		c.Window = c.K // window can't be smaller than the count
	}
	var files []string
	for _, home := range c.Home {
		m, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, m...)
	}
	sort.Strings(files)

	// One scan, bucket records per match substring.
	buckets := make([][]record.Record, len(c.Match))
	for _, f := range files {
		rs, _, _ := transcript.WalkFile(f)
		for _, r := range rs {
			if r.Tool != c.Tool {
				continue
			}
			field := matchField(r, c.Tool)
			for i, m := range c.Match {
				if strings.Contains(field, m) {
					buckets[i] = append(buckets[i], r)
				}
			}
		}
	}

	var reports []drift.Report
	for i, m := range c.Match {
		recs := buckets[i]
		sort.SliceStable(recs, func(a, b int) bool { return recs[a].Timestamp.Before(recs[b].Timestamp) })
		if len(recs) < 5 {
			fmt.Printf("pattern %q: only %d occurrences — too few for a replay\n", m, len(recs))
			continue
		}
		cut := int(float64(len(recs)) * c.TrainFrac)
		if cut < 1 {
			cut = 1
		}
		train, test := recs[:cut], recs[cut:]
		if c.InjectShift {
			test = injectShift(test)
		}
		rep := drift.Replay(m, c.Tool, train, test, c.K, c.Window, c.Promote, !c.JSON)
		reports = append(reports, rep)
	}

	if c.JSON {
		b, _ := json.MarshalIndent(reports, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	for _, r := range reports {
		fmt.Printf("\npattern %q [%s]  rule=%d-in-%d\n", r.Pattern, r.Tool, r.RuleM, r.RuleW)
		fmt.Printf("  train: N=%d determinism=%.2f promoted=%v\n", r.TrainN, r.TrainDeterminism, r.Promoted)
		fmt.Printf("  stream: N=%d servedCorrect=%d leaked=%d maxConsecutive=%d\n", r.StreamN, r.ServedCorrect, r.Leaked, r.MaxConsecutive)
		fmt.Printf("  → %s", strings.ToUpper(r.Decision))
		if r.DemotedAtIndex >= 0 {
			fmt.Printf(" at stream index %d", r.DemotedAtIndex)
		}
		fmt.Println()
	}
	return nil
}

func matchField(r record.Record, tool string) string {
	switch tool {
	case "Bash":
		s, _ := r.Args["command"].(string)
		return s
	default:
		s, _ := r.Args["file_path"].(string)
		return s
	}
}

// injectShift overwrites the back half of the stream with a single fixed,
// definitely-different output — a controlled clean distribution shift to
// confirm demotion fires on real-cadence data.
func injectShift(test []record.Record) []record.Record {
	out := make([]record.Record, len(test))
	copy(out, test)
	for i := len(out) / 2; i < len(out); i++ {
		out[i].Result = record.Output{Stdout: "<<INJECTED-SHIFT>>"}
	}
	return out
}
