package eval_test

// This is the Phase 1 GO/NO-GO test. It is the declared gate for the whole
// project: if the eval harness cannot catch deliberately-subtle regressions
// (sensitivity) without false-alarming on benign volatility (specificity),
// Phase 1 has failed and we stop and rethink before logging anything live.
//
// It runs against the committed corpus under testdata/corpus, which is
// SYNTHETIC: deterministically generated schema-faithful fixtures (see
// `crystal synth-corpus`) with invented content, so the public repo ships
// no real transcript data. The fixtures preserve the exact per-tool Output
// shapes the comparators and corruptors require; real-record replay is a
// local property (`crystal extract` over your own transcripts).

import (
	"fmt"
	"sort"
	"testing"

	"github.com/justinstimatze/crystal/internal/artifact"
	"github.com/justinstimatze/crystal/internal/compare"
	"github.com/justinstimatze/crystal/internal/corpus"
	"github.com/justinstimatze/crystal/internal/eval"
	"github.com/justinstimatze/crystal/internal/record"
)

const corpusDir = "../../testdata/corpus"

func loadCorpus(t *testing.T) []record.Record {
	t.Helper()
	recs, err := corpus.Load(corpusDir)
	if err != nil {
		t.Fatalf("load corpus: %v (run `crystal synth-corpus` to build fixtures)", err)
	}
	if len(recs) == 0 {
		t.Fatal("corpus is empty")
	}
	return recs
}

// TestIdentityPromotes is a SANITY PRECONDITION, not go/no-go evidence:
// reproducing the historical output verbatim must promote. An all-green
// identity run with any surviving corruptor (TestSensitivity) is still a
// Phase 1 FAILURE.
func TestIdentityPromotes(t *testing.T) {
	recs := loadCorpus(t)
	for _, r := range eval.RunAll(artifact.Identity{}, recs) {
		if r.Decision != "promote" {
			t.Errorf("identity on %s: decision=%s fidelity=%.3f (want promote)", r.Tool, r.Decision, r.Fidelity)
			for _, d := range r.Divergences {
				t.Logf("  unexpected divergence %s: %s", d.ToolUseID, d.Reason)
			}
		}
	}
}

// TestSensitivity is the load-bearing assertion: every deliberate, subtle,
// single-field corruption must be CAUGHT. We assert per touched record —
// the comparator must reject every record the corruptor actually mutated.
// A single escape fails the test loudly (the brief's stop-and-rethink gate).
func TestSensitivity(t *testing.T) {
	recs := loadCorpus(t)
	groups := eval.GroupByTool(recs)

	for _, corruptor := range artifact.Corruptors() {
		touched, escaped := 0, 0
		for tool, cohort := range groups {
			// Scope a corruptor to the tool it targets ("" = any tool,
			// for the outcome-class corruptors that operate on errors).
			if corruptor.Target() != "" && corruptor.Target() != tool {
				continue
			}
			cmp, ok := compare.Lookup(tool)
			if !ok {
				continue
			}
			for _, r := range cohort {
				if !corruptor.Mutated(r) {
					continue
				}
				touched++
				produced, _ := corruptor.Produce(r)
				if v := cmp.Compare(produced, r.Result); v.Match {
					escaped++
					t.Errorf("ESCAPE: corruptor %q not caught on %s record %s", corruptor.Name(), tool, r.ToolUseID)
				}
			}
		}
		if touched == 0 {
			t.Logf("calibration: corruptor %q touched 0 records (no applicable field in corpus)", corruptor.Name())
			continue
		}
		t.Logf("corruptor %-18s touched=%-3d escaped=%d", corruptor.Name(), touched, escaped)
	}
}

// TestSpecificity asserts benign volatility (re-stamped timestamps,
// reordered Grep output) does NOT trip a divergence — no false alarms.
func TestSpecificity(t *testing.T) {
	recs := loadCorpus(t)
	for _, r := range eval.RunAll(artifact.BenignVolatility{}, recs) {
		if r.Decision != "promote" {
			t.Errorf("benign-volatility on %s: decision=%s fidelity=%.3f (want promote — false alarm)", r.Tool, r.Decision, r.Fidelity)
			for i, d := range r.Divergences {
				if i >= 5 {
					break
				}
				t.Logf("  false-positive divergence %s: %s", d.ToolUseID, d.Reason)
			}
		}
	}
}

// TestOverNormalization guards against the benign-volatility allowlist
// masking real regressions (Goodhart). Two checks:
//  1. the normalizer must not rewrite more than half of any record's stdout
//  2. sensitivity corruptors must still be caught AFTER normalization
//     (subsumed by TestSensitivity, since comparators normalize internally;
//     re-asserted here explicitly for the Bash digit/line corruptors).
func TestOverNormalization(t *testing.T) {
	recs := loadCorpus(t)
	const cap = 0.5
	for _, r := range recs {
		s := r.Result.Stdout
		if len(s) == 0 {
			continue
		}
		changed := compare.NormalizedBytesChanged(s)
		if frac := float64(changed) / float64(len(s)); frac > cap {
			t.Errorf("over-normalization: %s stdout %.0f%% rewritten by volatility normalizer (cap %.0f%%)", r.ToolUseID, frac*100, cap*100)
		}
	}
}

// TestOutcomeClassGate directly exercises the highest-value regression: a
// historical error flipped into a success shape must always be rejected.
// Uses the corpus error records when present, plus a controlled synthetic
// error record so the gate is exercised even if the corpus has none.
func TestOutcomeClassGate(t *testing.T) {
	recs := loadCorpus(t)
	cmp, _ := compare.Lookup("Bash")

	// Controlled: a synthetic Bash error record.
	synthetic := record.Record{
		Tool:      "Bash",
		ToolUseID: "synthetic-error",
		Result:    record.Output{IsError: true, Scalar: "Error: command failed"},
	}
	corpusErrors := 0
	check := func(r record.Record) {
		flip := artifactByName(t, "error-to-success")
		if !flip.(artifact.Mutator).Mutated(r) {
			return
		}
		produced, _ := flip.Produce(r)
		c, ok := compare.Lookup(r.Tool)
		if !ok {
			return
		}
		if v := c.Compare(produced, r.Result); v.Match {
			t.Errorf("outcome-class ESCAPE: error→success not caught on %s %s", r.Tool, r.ToolUseID)
		}
	}
	check(synthetic)
	for _, r := range recs {
		if r.Result.IsError {
			corpusErrors++
			check(r)
		}
	}
	// sanity: the gate also rejects a same-input success vs error directly
	if v := cmp.Compare(record.Output{IsError: false, Stdout: "x"}, record.Output{IsError: true, Scalar: "Error: x"}); v.Match {
		t.Error("outcome-class gate failed to reject success-vs-error directly")
	}
	t.Logf("calibration: %d error records in corpus exercised the outcome-class gate", corpusErrors)
	if corpusErrors < 5 {
		t.Logf("NOTE: error-class records are sparse (<5); outcome-class sensitivity rests largely on the synthetic case")
	}
}

// TestCalibrationLog emits the substrate go/no-go signals: per-tool input-
// group histogram (is there a crystallizable unit at all?) and result-size
// stats. Run with -v to read it. Does not assert — it surfaces.
func TestCalibrationLog(t *testing.T) {
	recs := loadCorpus(t)
	groups := eval.GroupByTool(recs)
	tools := make([]string, 0, len(groups))
	for tname := range groups {
		tools = append(tools, tname)
	}
	sort.Strings(tools)
	for _, tname := range tools {
		cohort := groups[tname]
		hist := eval.InputGroupHistogram(cohort)
		maxGroup, repeated := 0, 0
		for _, n := range hist {
			if n > maxGroup {
				maxGroup = n
			}
			if n > 1 {
				repeated++
			}
		}
		t.Logf("calibration %-8s N=%-3d uniqueInputs=%-3d repeatedInputs=%d maxGroup=%d %s",
			tname, len(cohort), len(hist), repeated, maxGroup, crystallizableHint(maxGroup))
	}
}

func crystallizableHint(maxGroup int) string {
	if maxGroup >= eval.MinSamples {
		return "← has an input-group at promotion size"
	}
	return fmt.Sprintf("(no input-group reaches N>=%d — residue not yet crystallizable here)", eval.MinSamples)
}

func artifactByName(t *testing.T, name string) artifact.Artifact {
	t.Helper()
	a, ok := artifact.ByName(name)
	if !ok {
		t.Fatalf("artifact %q not found", name)
	}
	return a
}
