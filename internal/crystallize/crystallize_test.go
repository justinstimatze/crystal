package crystallize_test

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/crystallize"
	"github.com/justinstimatze/crystal/internal/record"
)

func bash(stdout string) record.Record {
	return record.Record{Tool: "Bash", Result: record.Output{Stdout: stdout}}
}

func repeat(stdout string, n int) []record.Record {
	out := make([]record.Record, n)
	for i := range out {
		out[i] = bash(stdout)
	}
	return out
}

// A constant-output pattern with enough samples promotes and serves the
// whole holdout cleanly — the happy-path proof of loop.
func TestPromotesAndServesClean(t *testing.T) {
	recs := repeat("ok", 100) // trainFrac 0.4 → 40 train, 60 holdout
	spec := crystallize.Run("const", recs, 0.4, 3, 10)
	if !spec.Promoted || spec.PromoteDecision != "promote" {
		t.Fatalf("want promote, got %s (det=%.2f)", spec.PromoteDecision, spec.TrainDeterminism)
	}
	if spec.ServeDecision != "served-clean" || spec.Leaked != 0 {
		t.Errorf("want served-clean/0 leaked, got %s/%d", spec.ServeDecision, spec.Leaked)
	}
	if spec.ServedOutput.Stdout != "ok" {
		t.Errorf("served output = %q, want ok", spec.ServedOutput.Stdout)
	}
}

// Clean in training, drifts in the holdout → promotes, then the windowed
// rule demotes. The full propose→promote→serve→demote arc.
func TestPromotesThenDemotesOnHoldoutDrift(t *testing.T) {
	recs := append(repeat("v1", 50), repeat("v2", 50)...) // first 50 clean (40 train + 10 holdout), then shift
	spec := crystallize.Run("shift", recs, 0.4, 3, 10)
	if !spec.Promoted {
		t.Fatalf("should promote on clean training, got %s", spec.PromoteDecision)
	}
	if spec.DemotedAtIndex < 0 || spec.ServeDecision != "demoted" {
		t.Errorf("should demote on holdout drift, got %s at %d", spec.ServeDecision, spec.DemotedAtIndex)
	}
}

// Too few samples → refused, nothing deployed.
func TestRefusesInsufficient(t *testing.T) {
	spec := crystallize.Run("tiny", repeat("ok", 10), 0.4, 3, 10) // 4 train < MinSamples
	if spec.Promoted || spec.PromoteDecision != "insufficient" {
		t.Errorf("want insufficient/refused, got %s promoted=%v", spec.PromoteDecision, spec.Promoted)
	}
}

// Non-deterministic training → rejected.
func TestRefusesNonDeterministic(t *testing.T) {
	var recs []record.Record
	for i := 0; i < 100; i++ {
		recs = append(recs, bash(string(rune('a'+i%26))+string(rune('0'+i%10)))) // mostly unique
	}
	spec := crystallize.Run("noisy", recs, 0.4, 3, 10)
	if spec.Promoted || spec.PromoteDecision != "reject" {
		t.Errorf("want reject, got %s (det=%.2f)", spec.PromoteDecision, spec.TrainDeterminism)
	}
}
