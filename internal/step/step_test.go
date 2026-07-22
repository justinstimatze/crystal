package step

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/record"
)

func rec(seq, turn int, tool string, args map[string]any, stdout string) record.Record {
	return record.Record{
		SessionID: "s1",
		Seq:       seq,
		Turn:      turn,
		Tool:      tool,
		Args:      args,
		Result:    record.Output{Stdout: stdout},
	}
}

// A move whose arguments were all sitting in the previous result is the
// mechanical band: the model acted as a clipboard.
func TestDerivedWhenArgsAppearInPriorResult(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Grep", map[string]any{"pattern": "handleEdit"}, "internal/server/handler.go\ninternal/server/route.go"),
		rec(1, 1, "Read", map[string]any{"file_path": "internal/server/handler.go"}, ""),
	}
	steps := Build(recs)
	if len(steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(steps))
	}
	if steps[0].Class != Derived {
		t.Errorf("class = %q, want %q (missing=%v)", steps[0].Class, Derived, steps[0].Missing)
	}
	if steps[0].DerivedFrac != 1 {
		t.Errorf("DerivedFrac = %v, want 1", steps[0].DerivedFrac)
	}
}

// Content the model invented is the judgment residual and must not be
// counted as shiftable.
func TestAuthoredWhenArgsAbsentFromPriorState(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "go test ./..."}, "ok  github.com/x/y  0.2s"),
		rec(1, 1, "Write", map[string]any{"content": "package main\n\nfunc Reconcile() error { return nil }"}, ""),
	}
	steps := Build(recs)
	if len(steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(steps))
	}
	if steps[0].Class != Authored {
		t.Errorf("class = %q, want %q", steps[0].Class, Authored)
	}
	if len(steps[0].Missing) == 0 {
		t.Error("want the authored text reported as Missing")
	}
}

func TestPartialWhenSomeArgsDerived(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Grep", map[string]any{"pattern": "Reconcile"}, "internal/sync/reconcile.go"),
		rec(1, 1, "Edit", map[string]any{
			"file_path":  "internal/sync/reconcile.go",
			"new_string": "a brand new body nobody has seen before",
		}, ""),
	}
	steps := Build(recs)
	if steps[0].Class != Partial {
		t.Fatalf("class = %q, want %q", steps[0].Class, Partial)
	}
	if steps[0].DerivedFrac <= 0 || steps[0].DerivedFrac >= 1 {
		t.Errorf("DerivedFrac = %v, want strictly between 0 and 1", steps[0].DerivedFrac)
	}
}

// The same move made twice is pure memoization — the cheapest substitution.
func TestRepeatDetectedWithinSession(t *testing.T) {
	args := map[string]any{"command": "go build ./..."}
	recs := []record.Record{
		rec(0, 1, "Bash", args, "ok"),
		rec(1, 1, "Read", map[string]any{"file_path": "unrelated/elsewhere.go"}, ""),
		rec(2, 1, "Bash", args, "ok"),
	}
	steps := Build(recs)
	if len(steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(steps))
	}
	if steps[1].Class != Repeat {
		t.Errorf("second step class = %q, want %q", steps[1].Class, Repeat)
	}
}

// Across a turn boundary the USER supplied the state, so the move is not
// attributable to the previous result and must not form a step.
func TestNoStepAcrossTurnBoundary(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "git status --short"}, "M internal/step/step.go"),
		rec(1, 2, "Read", map[string]any{"file_path": "internal/step/step.go"}, ""),
	}
	if steps := Build(recs); len(steps) != 0 {
		t.Fatalf("want 0 steps across a turn boundary, got %d", len(steps))
	}
}

func TestNoStepAcrossSessionBoundary(t *testing.T) {
	a := rec(0, 1, "Bash", map[string]any{"command": "git status --short"}, "internal/step/step.go")
	b := rec(1, 1, "Read", map[string]any{"file_path": "internal/step/step.go"}, "")
	b.SessionID = "s2"
	if steps := Build([]record.Record{a, b}); len(steps) != 0 {
		t.Fatalf("want 0 steps across a session boundary, got %d", len(steps))
	}
}

// Short argument strings are coincidence, not derivation.
func TestShortArgsIgnoredAsBare(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Bash", map[string]any{"command": "ls -la /tmp"}, "total 0"),
		rec(1, 1, "Bash", map[string]any{"command": "pwd"}, ""),
	}
	steps := Build(recs)
	if steps[0].Class != Bare {
		t.Errorf("class = %q, want %q", steps[0].Class, Bare)
	}
}

func TestSummarizeShiftable(t *testing.T) {
	p := Summarize([]Step{
		{Class: Derived, DerivedFrac: 1},
		{Class: Repeat, DerivedFrac: 1},
		{Class: Authored, DerivedFrac: 0},
		{Class: Partial, DerivedFrac: 0.5},
	})
	if p.N != 4 {
		t.Fatalf("N = %d, want 4", p.N)
	}
	if got := p.Shiftable(); got != 0.5 {
		t.Errorf("Shiftable = %v, want 0.5", got)
	}
	if p.MeanFrac != 0.625 {
		t.Errorf("MeanFrac = %v, want 0.625", p.MeanFrac)
	}
}

// The model's OWN prose must not count as available input: if it did, the
// model announcing its next move would make that move look mechanical by
// construction, inflating the shiftable ceiling.
func TestModelProseIsNotVisibleState(t *testing.T) {
	prev := rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok")
	prev.Context = "Now I will read internal/sync/reconcile.go"
	prev.Followup = "Next I will read internal/sync/reconcile.go"
	next := rec(1, 1, "Read", map[string]any{"file_path": "internal/sync/reconcile.go"}, "")

	steps := Build([]record.Record{prev, next})
	if steps[0].Class != Authored {
		t.Errorf("class = %q, want %q — the model's own prose leaked into visible state",
			steps[0].Class, Authored)
	}
}

// The USER's prompt is legitimate given input and must count.
func TestUserPromptIsVisibleState(t *testing.T) {
	prev := rec(0, 1, "Bash", map[string]any{"command": "go build ./..."}, "ok")
	prev.UserPrompt = "please open internal/sync/reconcile.go"
	next := rec(1, 1, "Read", map[string]any{"file_path": "internal/sync/reconcile.go"}, "")

	steps := Build([]record.Record{prev, next})
	if steps[0].Class != Derived {
		t.Errorf("class = %q, want %q — the user's prompt should count as input", steps[0].Class, Derived)
	}
}

// Streamer must agree with Build exactly — it is the bounded-memory path
// used on multi-GB transcripts, so a divergence would silently change every
// reported number.
func TestStreamerMatchesBuild(t *testing.T) {
	recs := []record.Record{
		rec(0, 1, "Grep", map[string]any{"pattern": "Reconcile"}, "internal/sync/reconcile.go"),
		rec(1, 1, "Read", map[string]any{"file_path": "internal/sync/reconcile.go"}, "package sync"),
		rec(2, 1, "Edit", map[string]any{"file_path": "internal/sync/reconcile.go", "new_string": "brand new content here"}, ""),
		rec(3, 2, "Bash", map[string]any{"command": "go test ./internal/sync/"}, "ok"),
		rec(4, 2, "Bash", map[string]any{"command": "go test ./internal/sync/"}, "ok"),
	}
	want := Build(recs)
	var got []Step
	st := NewStreamer(func(s Step) { got = append(got, s) })
	for _, r := range recs {
		st.Push(r)
	}
	if len(got) != len(want) {
		t.Fatalf("streamer produced %d steps, Build produced %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Class != want[i].Class || got[i].DerivedFrac != want[i].DerivedFrac {
			t.Errorf("step %d: streamer=%v/%v build=%v/%v", i,
				got[i].Class, got[i].DerivedFrac, want[i].Class, want[i].DerivedFrac)
		}
	}
}
