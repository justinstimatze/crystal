// Package eval replays a candidate artifact against historical Records and
// decides whether it reproduces the frontier's behavior faithfully enough
// to promote.
//
// The promote gate is intentionally strict and fails loud:
//   - comparator must be registered for the tool (else "unverifiable")
//   - at least MinSamples records (else "insufficient")
//   - fidelity >= PromoteThreshold (else "reject")
//
// Demotion is more aggressive than promotion: anything short of a clean
// promote refuses to crystallize.
package eval

import (
	"encoding/json"
	"sort"

	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/record"
)

const (
	// PromoteThreshold is the minimum fidelity to crystallize a pattern.
	PromoteThreshold = 0.95
	// MinSamples is the minimum cohort size for a meaningful decision.
	MinSamples = 30
)

// Divergence localizes a single failed comparison.
type Divergence struct {
	ToolUseID  string  `json:"tool_use_id"`
	Reason     string  `json:"reason"`
	Similarity float64 `json:"similarity"`
}

// EvalReport is the outcome of replaying one artifact over one tool's
// cohort.
type EvalReport struct {
	Tool        string       `json:"tool"`
	Artifact    string       `json:"artifact"`
	N           int          `json:"n"`
	Matched     int          `json:"matched"`
	Fidelity    float64      `json:"fidelity"`
	Decision    string       `json:"decision"` // promote | reject | unverifiable | insufficient
	Divergences []Divergence `json:"divergences,omitempty"`
}

// Run replays a over the given single-tool cohort and returns its report.
func Run(a artifact.Artifact, tool string, recs []record.Record) EvalReport {
	rep := EvalReport{Tool: tool, Artifact: a.Name(), N: len(recs)}
	cmp, ok := compare.Lookup(tool)
	if !ok {
		rep.Decision = "unverifiable"
		return rep
	}
	for _, r := range recs {
		produced, err := a.Produce(r)
		if err != nil {
			rep.Divergences = append(rep.Divergences, Divergence{r.ToolUseID, "produce error: " + err.Error(), 0})
			continue
		}
		v := cmp.Compare(produced, r.Result)
		if v.Match {
			rep.Matched++
		} else {
			rep.Divergences = append(rep.Divergences, Divergence{r.ToolUseID, v.Reason, v.Similarity})
		}
	}
	if rep.N > 0 {
		rep.Fidelity = float64(rep.Matched) / float64(rep.N)
	}
	switch {
	case rep.N < MinSamples:
		rep.Decision = "insufficient"
	case rep.Fidelity >= PromoteThreshold:
		rep.Decision = "promote"
	default:
		rep.Decision = "reject"
	}
	return rep
}

// RunAll groups the corpus by tool and runs a over each group.
func RunAll(a artifact.Artifact, corpus []record.Record) []EvalReport {
	groups := GroupByTool(corpus)
	tools := make([]string, 0, len(groups))
	for t := range groups {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	out := make([]EvalReport, 0, len(tools))
	for _, t := range tools {
		out = append(out, Run(a, t, groups[t]))
	}
	return out
}

// GroupByTool partitions a corpus by tool name.
func GroupByTool(corpus []record.Record) map[string][]record.Record {
	g := map[string][]record.Record{}
	for _, r := range corpus {
		g[r.Tool] = append(g[r.Tool], r)
	}
	return g
}

// InputSignature is the canonical key identifying records that share an
// input — the actual crystallizable unit (not the tool). Records with the
// same signature and a single-valued output are the crystallization
// candidates the GATE will eventually find.
func InputSignature(r record.Record) string {
	if len(r.Args) == 0 {
		return r.Tool + ":{}"
	}
	b, err := json.Marshal(r.Args) // map marshals with sorted keys
	if err != nil {
		return r.Tool + ":?"
	}
	return r.Tool + ":" + string(b)
}

// InputGroupHistogram counts how many records share each input signature
// within a cohort. Feeds the calibration log's "is there a crystallizable
// unit at all" question.
func InputGroupHistogram(recs []record.Record) map[string]int {
	h := map[string]int{}
	for _, r := range recs {
		h[InputSignature(r)]++
	}
	return h
}
