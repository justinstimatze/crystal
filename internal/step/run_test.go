package step

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/record"
)

func TestRunProfileCountsConsecutiveClassRuns(t *testing.T) {
	// Two Authored moves back to back is one run of length 2, not two runs
	// of length 1 — this is the whole point of the isolated-vs-clustered
	// distinction.
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(1, 1, "Write", map[string]any{"content": "brand new content one nobody has seen"}, ""),
		rec(2, 1, "Write", map[string]any{"content": "brand new content two nobody has seen"}, ""),
	}
	p := NewRunProfile()
	for _, s := range Build(recs) {
		p.Add(s)
	}
	p.Flush()
	if p.Runs[Authored][2] != 1 {
		t.Fatalf("want one run of length 2, got histogram %v", p.Runs[Authored])
	}
	if got := p.IsolatedFrac(Authored); got != 0 {
		t.Errorf("IsolatedFrac = %v, want 0 (the only run has length 2)", got)
	}
}

func TestRunProfileIsolatedAuthoredStep(t *testing.T) {
	// One Authored step sandwiched between non-Authored steps is a run of
	// length 1 for Authored — the case the episode hypothesis is betting on.
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(1, 1, "Write", map[string]any{"content": "a totally novel fix nobody wrote before"}, ""),
		rec(2, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"), // repeats rec0's move
	}
	p := NewRunProfile()
	for _, s := range Build(recs) {
		p.Add(s)
	}
	p.Flush()
	if p.Runs[Authored][1] != 1 {
		t.Fatalf("want one isolated Authored run, got histogram %v", p.Runs[Authored])
	}
	if got := p.IsolatedFrac(Authored); got != 1 {
		t.Errorf("IsolatedFrac = %v, want 1", got)
	}
}

func TestRunProfileGapRequiresBothBounds(t *testing.T) {
	// Mechanical steps AFTER the only Authored step have no closing Authored
	// step, so they must not be counted as a bounded episode interior.
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(1, 1, "Write", map[string]any{"content": "a totally novel fix nobody wrote before"}, ""),
		rec(2, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(3, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
	}
	p := NewRunProfile()
	for _, s := range Build(recs) {
		p.Add(s)
	}
	p.Flush()
	if n, _ := p.GapCount(); n != 0 {
		t.Fatalf("want 0 bounded gaps (no closing Authored step), got %d: %v", n, p.Gaps)
	}
}

func TestRunProfileGapBetweenTwoAuthoredSteps(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(1, 1, "Write", map[string]any{"content": "first novel fix nobody wrote before"}, ""),  // Authored, opens gap
		rec(2, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),                      // repeats rec0
		rec(3, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),                      // repeats rec0 again
		rec(4, 1, "Write", map[string]any{"content": "second novel fix nobody wrote before"}, ""), // Authored, closes gap of len 2
	}
	p := NewRunProfile()
	for _, s := range Build(recs) {
		p.Add(s)
	}
	p.Flush()
	if p.Gaps[2] != 1 {
		t.Fatalf("want one bounded gap of length 2, got %v", p.Gaps)
	}
	if got := p.MeanGap(); got != 2 {
		t.Errorf("MeanGap = %v, want 2", got)
	}
}

func TestRunProfileResetsAcrossSessionAndTurnBoundaries(t *testing.T) {
	a := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok"),
		rec(1, 1, "Write", map[string]any{"content": "first novel fix nobody wrote before"}, ""),
	}
	b := rec(0, 1, "Write", map[string]any{"content": "second novel fix in a different session"}, "")
	b.SessionID = "s2"

	p := NewRunProfile()
	for _, s := range Build(a) {
		p.Add(s)
	}
	// b has no paired step of its own (single record, new session) — feeding
	// it through Build alongside a would produce zero additional steps since
	// a session boundary blocks step formation, so directly exercise the
	// adjacency break by resetting via a fresh, unrelated first step.
	fresh := Build([]record.Record{
		rec(0, 1, "Grep", map[string]any{"pattern": "Reconcile"}, "internal/sync/reconcile.go"),
		rec(1, 1, "Read", map[string]any{"file_path": "internal/sync/reconcile.go"}, "package sync"),
	})
	for _, s := range fresh {
		p.Add(s)
	}
	p.Flush()
	// The isolated Authored run from `a` and the Derived step from `fresh`
	// must not merge into one run just because they were fed consecutively.
	if p.Runs[Authored][1] != 1 {
		t.Fatalf("want the Authored run from `a` preserved as length 1, got %v", p.Runs[Authored])
	}
	if p.Runs[Derived][1] != 1 {
		t.Fatalf("want the Derived step from `fresh` counted on its own, got %v", p.Runs[Derived])
	}
}
