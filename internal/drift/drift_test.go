package drift_test

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/drift"
	"github.com/justinstimatze/crystal/internal/record"
)

// bashOut builds a Bash success record with the given stdout.
func bashOut(stdout string) record.Record {
	return record.Record{Tool: "Bash", Result: record.Output{Stdout: stdout}}
}

func train(stdout string, n int) []record.Record {
	out := make([]record.Record, n)
	for i := range out {
		out[i] = bashOut(stdout)
	}
	return out
}

// Scenario A: a genuinely stable pattern never demotes and never leaks.
// (Specificity of the drift mechanism — no false demotion.)
func TestStablePatternServesClean(t *testing.T) {
	tr := train("ok", 40)
	test := train("ok", 60)
	rep := drift.Replay("stable", "Bash", tr, test, 3, 3, 0.95, false)
	if !rep.Promoted {
		t.Fatalf("stable pattern should promote (det=%.2f)", rep.TrainDeterminism)
	}
	if rep.Decision != "served-clean" || rep.Leaked != 0 || rep.DemotedAtIndex != -1 {
		t.Errorf("stable: decision=%s leaked=%d demotedAt=%d (want served-clean/0/-1)", rep.Decision, rep.Leaked, rep.DemotedAtIndex)
	}
	if rep.ServedCorrect != 60 {
		t.Errorf("stable: servedCorrect=%d want 60", rep.ServedCorrect)
	}
}

// Scenario B: a clean shift (the pattern's output changes and stays
// changed) is demoted within K, with leakage bounded by K. This is the
// brief's success case: "drift detector fires on a distribution shift."
func TestCleanShiftDemotesWithinK(t *testing.T) {
	tr := train("v1", 40)
	test := append(train("v1", 10), train("v2", 50)...) // shift at index 10, stays shifted
	rep := drift.Replay("clean-shift", "Bash", tr, test, 3, 3, 0.95, false)
	if rep.Decision != "demoted" {
		t.Fatalf("clean shift should demote, got %s", rep.Decision)
	}
	if rep.DemotedAtIndex != 12 { // 10 clean, then 3 consecutive wrong → demote on the 3rd (index 12)
		t.Errorf("demotedAtIndex=%d want 12", rep.DemotedAtIndex)
	}
	if rep.Leaked != 3 {
		t.Errorf("clean shift leaked=%d want 3 (bounded by K)", rep.Leaked)
	}
}

// Scenario C — THE risk finding: intermittent (flapping) drift never
// accumulates K consecutive divergences, so the brief's consecutive-K rule
// NEVER demotes — while wrong outputs leak the entire time. This is the
// undercity failure mode the experiment was built to expose.
func TestIntermittentDriftEvadesConsecutiveRule(t *testing.T) {
	tr := train("v1", 40)
	// Alternate correct/wrong forever: never 3-in-a-row wrong.
	var test []record.Record
	for i := 0; i < 60; i++ {
		if i%2 == 0 {
			test = append(test, bashOut("v1")) // correct
		} else {
			test = append(test, bashOut("v2")) // wrong
		}
	}
	// consecutive-K rule (W==M==3): evaded.
	rep := drift.Replay("intermittent", "Bash", tr, test, 3, 3, 0.95, false)
	if rep.DemotedAtIndex != -1 {
		t.Errorf("intermittent drift unexpectedly demoted at %d", rep.DemotedAtIndex)
	}
	if rep.Decision != "leaked-without-demote" {
		t.Errorf("decision=%s want leaked-without-demote", rep.Decision)
	}
	if rep.Leaked == 0 {
		t.Fatal("expected leaked > 0 — the whole point is silent-wrong output")
	}
	if rep.MaxConsecutive >= 3 {
		t.Errorf("maxConsecutive=%d should stay < K=3 for flapping drift", rep.MaxConsecutive)
	}
	t.Logf("RISK CONFIRMED: consecutive-K leaked %d wrong outputs (max run %d < K=3), never demoted",
		rep.Leaked, rep.MaxConsecutive)
}

// THE FIX: a sliding-window rate rule (M divergences in last W) demotes the
// same intermittent drift the consecutive-K rule let leak forever — and
// bounds the leak.
func TestWindowedRuleCatchesIntermittentDrift(t *testing.T) {
	tr := train("v1", 40)
	var test []record.Record
	for i := 0; i < 60; i++ {
		if i%2 == 0 {
			test = append(test, bashOut("v1"))
		} else {
			test = append(test, bashOut("v2"))
		}
	}
	// 3 divergences within a sliding window of 10.
	rep := drift.Replay("intermittent", "Bash", tr, test, 3, 10, 0.95, false)
	if rep.Decision != "demoted" {
		t.Fatalf("windowed rule should demote intermittent drift, got %s (leaked=%d)", rep.Decision, rep.Leaked)
	}
	if rep.Leaked > 6 {
		t.Errorf("windowed rule leaked=%d — should bound leakage near M; alternating 3-in-10 should fire by ~index 5", rep.Leaked)
	}
	t.Logf("FIX VERIFIED: windowed rule (3-in-10) demoted intermittent drift at index %d, leaked only %d (vs unbounded for consecutive-K)",
		rep.DemotedAtIndex, rep.Leaked)
}
