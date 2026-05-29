package cmd

import "testing"

// decideHook is the pure core of the live PreToolUse hook. These tests assert
// the two behaviors the live hook must get right: (1) it SERVES covered commands
// (injecting the deterministic category, 0 model calls) and stays silent on the
// residual; (2) it DEMOTES on windowed coverage collapse and then defers
// everything — the live demote-on-drift, exercised purely.

func TestDecideHookServesAndDefers(t *testing.T) {
	st := &hookState{DemotedAtTotal: -1}

	// A covered command → served with its category injected.
	d := decideHook(st, "git status", 3, 5)
	if !d.served || d.category != "git" {
		t.Fatalf("covered command: want served=git, got served=%v cat=%q", d.served, d.category)
	}
	if d.additionalContext == "" {
		t.Fatal("served command must inject additionalContext")
	}

	// A single residual command → silent defer, no demotion (window not collapsed).
	d = decideHook(st, "xyzzy --frobnicate", 3, 5)
	if d.served || d.demotedNow || d.additionalContext != "" {
		t.Fatalf("residual command: want silent defer, got %+v", d)
	}
	if st.Served != 1 || st.Deferred != 1 {
		t.Fatalf("tallies: want served=1 deferred=1, got served=%d deferred=%d", st.Served, st.Deferred)
	}
}

func TestDecideHookDemotesOnCoverageCollapse(t *testing.T) {
	st := &hookState{DemotedAtTotal: -1}
	// Three covered commands keep coverage healthy (no demote at M=3, W=5).
	for _, c := range []string{"git add .", "go test ./...", "rg foo"} {
		if d := decideHook(st, c, 3, 5); !d.served {
			t.Fatalf("expected %q served", c)
		}
	}
	if st.Demoted {
		t.Fatal("must not demote while coverage is healthy")
	}
	// Now a burst of uncovered container commands. With M=3-in-5 the third
	// uncovered one (stream index 6, the 3rd container cmd) collapses the window.
	burst := driftCommands // docker/podman/kubectl — none covered by the rules
	demotedAt := -1
	for i, c := range burst {
		d := decideHook(st, c, 3, 5)
		if d.demotedNow {
			demotedAt = i
			break
		}
	}
	if demotedAt != 2 {
		t.Fatalf("want demotion on the 3rd uncovered command (index 2 of the burst), got %d", demotedAt)
	}
	if !st.Demoted || st.DemotedAtTotal != 6 {
		t.Fatalf("want Demoted=true at total=6 (3 covered + 3 uncovered), got Demoted=%v total=%d", st.Demoted, st.DemotedAtTotal)
	}
	// After demotion, every further command is a silent defer (chore on the model).
	d := decideHook(st, "git commit -m x", 3, 5) // covered, but tier is demoted
	if d.served || d.additionalContext != "" {
		t.Fatalf("post-demotion must defer silently even on a covered command, got %+v", d)
	}
}
