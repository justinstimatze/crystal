package cmd

import (
	"strings"
	"testing"
)

// TestProcSignature pins the coarse transcript signature: leading command +
// plain subcommand word, all args/flags/refs dropped, non-command leads excluded.
func TestProcSignature(t *testing.T) {
	tests := map[string]string{
		"git commit-tree HEAD^{tree} -m x": "git commit-tree", // ref dropped, subcommand kept
		"git reset --hard NEW":             "git reset",        // flag ends the chain
		"gh api repos/owner/r --paginate":  "gh api",
		"git add -A":                       "git add",
		"docker build -t app .":            "docker build",
		"rg foo | head":                    "", // rg is not a crystallizable lead
		"FOO=1 git push":                   "", // assignment lead, not a command
		"":                                 "",
	}
	for in, want := range tests {
		if got := procSignature(in); got != want {
			t.Errorf("procSignature(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSplitShellSequentialOnly confirms compound commands split on &&/;/|| but
// NOT on pipes (a pipeline is one logical step).
func TestSplitShellSequentialOnly(t *testing.T) {
	got := splitShell("git add -A && git commit -m x ; git push")
	if len(got) != 3 {
		t.Fatalf("want 3 steps, got %d: %v", len(got), got)
	}
	if pipe := splitShell("rg foo | head -5"); len(pipe) != 1 {
		t.Errorf("pipeline should stay one step, got %v", pipe)
	}
}

// TestSessionSignaturesCollapsesRuns confirms a run of the same signature folds to
// one step (so "git add x; git add y" is a single step, not two).
func TestSessionSignaturesCollapsesRuns(t *testing.T) {
	raw := []string{"git add foo.go", "git add bar.go && git commit -m x", "git commit --amend"}
	got := sessionSignatures(raw)
	want := []string{"git add", "git commit"} // both git-add runs collapse; both git-commit collapse
	if strings.Join(got.sigs, ",") != strings.Join(want, ",") {
		t.Errorf("sigs = %v, want %v", got.sigs, want)
	}
}

// TestNovelFilter separates everyday git CRUD from distinctive ceremonies.
func TestNovelFilter(t *testing.T) {
	if hasUncommonStep([]string{"git add", "git commit", "git push"}) {
		t.Error("all-CRUD sequence should NOT count as novel")
	}
	if !hasUncommonStep([]string{"go build", "git add"}) {
		t.Error("a go-build step makes the sequence novel")
	}
	if hasTwoDistinct([]string{"git add", "git add"}) {
		t.Error("a→a is not two-distinct")
	}
	if !hasTwoDistinct([]string{"git add", "git commit"}) {
		t.Error("a→b is two-distinct")
	}
}
