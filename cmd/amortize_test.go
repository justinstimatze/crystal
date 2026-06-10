package cmd

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/llm"
)

func TestTokenCostUSD(t *testing.T) {
	// Opus: $5/1M in, $25/1M out. 1M in + 1M out = $30.
	if got := tokenCostUSD(llm.ModelOpus, 1_000_000, 1_000_000); got != 30.0 {
		t.Errorf("opus cost = %.4f, want 30.0", got)
	}
	// Haiku: $1/1M in, $5/1M out.
	if got := tokenCostUSD(llm.ModelHaiku, 1_000_000, 1_000_000); got != 6.0 {
		t.Errorf("haiku cost = %.4f, want 6.0", got)
	}
	// Unknown model defaults to Opus pricing (conservative).
	if got := tokenCostUSD("mystery", 1_000_000, 0); got != 5.0 {
		t.Errorf("unknown-model cost = %.4f, want 5.0 (opus in-rate)", got)
	}
}

func TestSubsampleStr(t *testing.T) {
	in := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	got := subsampleStr(in, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Evenly spaced across the whole slice, deterministic, includes the head.
	if got[0] != "a" {
		t.Errorf("first = %q, want a", got[0])
	}
	// n >= len returns the input unchanged.
	if len(subsampleStr(in, 100)) != len(in) {
		t.Error("oversized n should return the full slice")
	}
	if len(subsampleStr(in, 0)) != len(in) {
		t.Error("n<=0 should return the full slice")
	}
}
