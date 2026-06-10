package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/record"
)

// SynthCorpusCmd generates the committed test corpus deterministically with
// INVENTED, schema-faithful content. The fixtures preserve the per-tool
// Output shapes the comparators and corruptors require — multi-line Bash
// stdout with a non-volatile leading digit (so bash-flipdigit isn't masked
// by the clock normalizer), Edit/Write structuredPatch hunks + newString, a
// Grep filename set, Read file bodies, and per-tool error envelopes — but
// contain zero real transcript content. That keeps the public repo free of
// private-project names, paths, and source while still exercising the
// Phase-1 eval gate in CI. Real-record replay is a LOCAL property: users run
// `crystal extract` over their own transcripts.
type SynthCorpusCmd struct {
	Out     string `help:"Output corpus dir." default:"testdata/corpus"`
	PerTool int    `help:"Success-class synthetic records per tool (must keep N >= eval.MinSamples)." default:"45"`
	Errors  int    `help:"Error-class records per tool (exercise the outcome-class gate)." default:"5"`
}

func (c *SynthCorpusCmd) Run() error {
	recs := synthCorpus(c.PerTool, c.Errors)
	if err := corpus.Save(c.Out, recs); err != nil {
		return err
	}
	fmt.Printf("synth-corpus: wrote %d synthetic records to %s (%d success + %d error per tool, 5 tools)\n",
		len(recs), c.Out, c.PerTool, c.Errors)
	return nil
}

// synthBase is a fixed epoch so generation is deterministic (no time.Now).
var synthBase = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func synthRecord(tool string, i int, out record.Output) record.Record {
	id := fmt.Sprintf("synthetic-%s-%04d", tool, i)
	return record.Record{
		SessionID: id,
		Repo:      "$HOME/Documents/example",
		GitBranch: "main",
		Timestamp: synthBase.Add(time.Duration(i) * time.Minute),
		Context:   "synthetic schema-faithful fixture (no real content)",
		Tool:      tool,
		Args:      map[string]any{"n": i},
		Result:    out,
		ToolUseID: id,
	}
}

// synthHunks builds a 2-hunk structuredPatch so edit-drophunk has something
// to drop and the comparator can still distinguish the result.
func synthHunks(i int) json.RawMessage {
	hunks := []map[string]any{
		{"oldStart": i + 1, "oldLines": 1, "newStart": i + 1, "newLines": 1,
			"lines": []string{fmt.Sprintf("-const N = %d", i), fmt.Sprintf("+const N = %d", i+1)}},
		{"oldStart": i + 10, "oldLines": 0, "newStart": i + 11, "newLines": 1,
			"lines": []string{fmt.Sprintf("+// note %d", i)}},
	}
	b, _ := json.Marshal(hunks)
	return b
}

func synthCorpus(perTool, errs int) []record.Record {
	var recs []record.Record
	for i := 0; i < perTool; i++ {
		// Bash: a non-volatile leading count ("%d records") precedes the clock
		// so bash-flipdigit mutates a digit the volatility normalizer does NOT
		// collapse; the clock exists so benign-volatility (clock-swap) is
		// exercised and must still promote.
		recs = append(recs, synthRecord("Bash", i, record.Output{
			Stdout: fmt.Sprintf("scan complete: %d records matched\nstatus ok at 12:%02d:30\nchecksum %d\n", 40+i, i%60, 7000+i),
		}))
		recs = append(recs, synthRecord("Edit", i, record.Output{
			NewString:       fmt.Sprintf("func handler%d() error {\n\treturn nil\n}\n", i),
			StructuredPatch: synthHunks(i),
		}))
		recs = append(recs, synthRecord("Write", i, record.Output{
			NewString:       fmt.Sprintf("package demo\n\n// generated file %d\nconst Limit = %d\n", i, 100+i),
			StructuredPatch: synthHunks(i),
		}))
		recs = append(recs, synthRecord("Grep", i, record.Output{
			Filenames: []string{
				fmt.Sprintf("internal/pkg/a%d.go", i),
				fmt.Sprintf("internal/pkg/b%d.go", i),
				fmt.Sprintf("cmd/c%d.go", i),
			},
			NumFiles: 3,
		}))
		recs = append(recs, synthRecord("Read", i, record.Output{
			File: fmt.Sprintf("// file %d\npackage demo\n\nfunc Demo%d() int {\n\treturn %d\n}\n", i, i, i),
		}))
	}
	// Error-class records per tool: exercise the outcome-class gate and the
	// scalar-mutate / error-to-success corruptors (target "" = any tool).
	for _, tool := range []string{"Bash", "Edit", "Write", "Grep", "Read"} {
		for j := 0; j < errs; j++ {
			recs = append(recs, synthRecord(tool, 1000+j, record.Output{
				IsError: true,
				Scalar:  fmt.Sprintf("Error: %s failed on item %d", tool, j),
			}))
		}
	}
	return recs
}
