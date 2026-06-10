package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/eval"
)

// EvalCmd replays a synthetic artifact over a corpus and prints reports.
type EvalCmd struct {
	Corpus   string `help:"Corpus dir to replay against." default:"testdata/corpus"`
	Artifact string `help:"Synthetic artifact name (identity, benign-volatility, or a corruptor)." default:"identity"`
	JSON     bool   `help:"Emit reports as JSON."`
}

func (c *EvalCmd) Run() error {
	a, ok := artifact.ByName(c.Artifact)
	if !ok {
		return usageError{fmt.Errorf("unknown artifact %q; choose one of: %s", c.Artifact, strings.Join(artifact.Names(), ", "))}
	}
	recs, err := corpus.Load(c.Corpus)
	if err != nil {
		return usageError{fmt.Errorf("loading corpus %q: %w", c.Corpus, err)}
	}
	if len(recs) == 0 {
		return usageError{fmt.Errorf("corpus %q is empty — run `crystal extract` first", c.Corpus)}
	}

	reports := eval.RunAll(a, recs)
	if c.JSON {
		b, _ := json.MarshalIndent(reports, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("artifact: %s\n", a.Name())
	for _, r := range reports {
		fmt.Printf("  %-8s N=%-3d fidelity=%.3f  %s\n", r.Tool, r.N, r.Fidelity, strings.ToUpper(r.Decision))
		for i, d := range r.Divergences {
			if i >= 3 {
				fmt.Printf("      … %d more divergences\n", len(r.Divergences)-3)
				break
			}
			fmt.Printf("      ✗ %s: %s\n", short(d.ToolUseID), d.Reason)
		}
	}
	return nil
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
