package cmd

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/library"
)

// decideLibraryHook is the pure core of the live serve hook. These tests assert
// the behaviors the live hook must get right ACROSS the disk round-trip that a
// fresh-process-per-event deployment forces: it SERVES a confident match
// (injecting the recipe), it holds COOLDOWN across the state boundary (the same
// entry does not re-inject on the next event even though that is a separate
// "process" — modeled here by reloading the library and restoring the persisted
// state each step), it honors the TOO-MUCH guard, and a DEMOTION persisted in
// the state keeps the entry out of service.

func hookTestEntries() []library.Entry {
	return []library.Entry{
		{Name: "guard", Rung: "code", Match: []string{"git", "add", "-a"}, Avoid: []string{"explicit"}, MinConf: 0.5, Artifact: "stage explicit paths"},
		{Name: "struct", Rung: "recipe", Match: []string{"entity", "struct", "field", "go", "type"}, Avoid: []string{"method"}, MinConf: 0.4, Artifact: "render the struct via the template"},
	}
}

// freshServe models one real hook process: load the library anew, restore the
// persisted serve state, decide, and return the decision plus the state to save.
func freshServe(st library.ServeState, text string) (library.Decision, library.ServeState) {
	lib := library.New(hookTestEntries())
	return decideLibraryHook(lib, st, text)
}

func TestLibraryHookServesConfidentMatch(t *testing.T) {
	d, next := freshServe(library.ServeState{}, "please git add -a and commit")
	if !d.Served() || d.Entry.Name != "guard" {
		t.Fatalf("expected serve guard, got %s %v", d.Outcome, d.Entry)
	}
	if serveInjection(d) == "" {
		t.Fatal("a served entry must inject additionalContext")
	}
	if next.Step != 1 {
		t.Fatalf("step must advance to 1, got %d", next.Step)
	}
}

func TestLibraryHookAbstainsSilently(t *testing.T) {
	d, _ := freshServe(library.ServeState{}, "just some unrelated chatter about the weather")
	if d.Served() {
		t.Fatalf("no match must abstain, got %s", d.Outcome)
	}
	if serveInjection(d) != "" {
		t.Fatal("an abstain must inject nothing (silent defer)")
	}
}

// TestLibraryHookCooldownPersistsAcrossProcesses is the load-bearing claim: the
// same entry, matched again on the very next event, abstains on cooldown — and
// that only works because the cooldown window round-tripped through the
// persisted state between two separate "processes".
func TestLibraryHookCooldownPersistsAcrossProcesses(t *testing.T) {
	// A partial match (2/3 = 0.67) clears min_conf 0.5 but not the cooldown bar
	// 0.5+0.3=0.8, so it serves once then abstains on cooldown.
	d1, st1 := freshServe(library.ServeState{}, "git add now")
	if !d1.Served() {
		t.Fatalf("first event should serve, got %s (conf %.2f)", d1.Outcome, d1.Confidence)
	}
	// Next process: a brand-new library, restoring st1 from disk.
	d2, _ := freshServe(st1, "git add now")
	if d2.Outcome != "abstain-cooldown" {
		t.Fatalf("second event (separate process, restored state) must abstain on cooldown, got %s", d2.Outcome)
	}
}

func TestLibraryHookTooMuchGuard(t *testing.T) {
	d, _ := freshServe(library.ServeState{}, "git add -a but only the explicit files")
	if d.Outcome != "abstain-too-much" {
		t.Fatalf("the 'explicit' avoid token must trip the too-much guard, got %s", d.Outcome)
	}
}

// TestLibraryHookDemotionPersists confirms a demotion carried in the persisted
// state keeps the entry out of service in the next process (the serve-layer
// demote the control op / re-author loop drives).
func TestLibraryHookDemotionPersists(t *testing.T) {
	d, _ := freshServe(library.ServeState{Demoted: []string{"guard"}}, "please git add -a")
	if d.Served() {
		t.Fatalf("a demoted entry must not serve, got %s", d.Outcome)
	}
}

// TestLibraryHookSurfaceDetection asserts the two-surface finding: the prompt
// prose (UserPromptSubmit) carries intent tokens and fires the intent entry,
// while a Bash command (PreToolUse) only fires an entry whose triggers appear in
// the command itself (the git-add guard) — the intent entry stays silent there.
func TestLibraryHookSurfaceDetection(t *testing.T) {
	// UserPromptSubmit: intent prose fires the struct entry.
	ev := libHookEvent{HookEventName: "UserPromptSubmit", Prompt: "generate a Go struct type for this entity with fields"}
	text, surface := ev.serveContext()
	if surface != "UserPromptSubmit" {
		t.Fatalf("prompt event must resolve to UserPromptSubmit, got %s", surface)
	}
	if d, _ := freshServe(library.ServeState{}, text); !d.Served() || d.Entry.Name != "struct" {
		t.Fatalf("prompt intent must serve the struct entry, got %s %v", d.Outcome, d.Entry)
	}

	// PreToolUse: a bash command carries no struct-intent tokens → the struct
	// entry cannot fire; only the git-add guard would (proves the narrow surface).
	var pt libHookEvent
	pt.HookEventName = "PreToolUse"
	pt.ToolName = "Bash"
	pt.ToolInput.Command = "go build ./internal/mytype"
	ptText, ptSurface := pt.serveContext()
	if ptSurface != "PreToolUse" {
		t.Fatalf("tool event must resolve to PreToolUse, got %s", ptSurface)
	}
	if d, _ := freshServe(library.ServeState{}, ptText); d.Served() {
		t.Fatalf("a plain build command must not fire an intent entry, got serve of %v", d.Entry)
	}
}
