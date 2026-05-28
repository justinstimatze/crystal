package measure_test

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/measure"
	"github.com/justinstimatze/crystal/internal/record"
)

func bash(cmd, stdout string) record.Record {
	return record.Record{
		Tool:   "Bash",
		Args:   map[string]any{"command": cmd},
		Result: record.Output{Stdout: stdout},
	}
}

// A frequent command with a CONSTANT output is crystallizable; a frequent
// command with a VARYING output is not. The accumulator must distinguish
// them at exact granularity via the determinism metric.
func TestAccumulatorDeterminism(t *testing.T) {
	acc := measure.New()
	for i := 0; i < 40; i++ {
		acc.Add(bash("heartbeat", "ok")) // constant output → det 1.0
	}
	for i := 0; i < 40; i++ {
		acc.Add(bash("git status", string(rune('a'+i)))) // unique each time → det low
	}

	reports := acc.Report(30, 0.95)
	exact := reports[0]
	if exact.Name != "exact" {
		t.Fatalf("first granularity = %q, want exact", exact.Name)
	}
	if len(exact.Crystallizable) != 1 {
		t.Fatalf("crystallizable groups = %d, want 1 (only the constant-output command)", len(exact.Crystallizable))
	}
	got := exact.Crystallizable[0]
	if got.N != 40 || got.Determinism < 0.99 {
		t.Errorf("crystallizable group: N=%d det=%.2f, want N=40 det~1.0", got.N, got.Determinism)
	}
}

// The outcome class is part of the fingerprint: the same command emitting
// success vs error must split into distinct output classes, dropping
// determinism below the gate.
func TestAccumulatorOutcomeClassSplitsDeterminism(t *testing.T) {
	acc := measure.New()
	for i := 0; i < 20; i++ {
		acc.Add(bash("flaky", "ok"))
	}
	for i := 0; i < 20; i++ {
		r := bash("flaky", "")
		r.Result = record.Output{IsError: true, Scalar: "Error: boom"}
		acc.Add(r)
	}
	exact := acc.Report(30, 0.95)[0]
	if len(exact.Crystallizable) != 0 {
		t.Errorf("a 50/50 success/error command must not be crystallizable, got %d", len(exact.Crystallizable))
	}
}
