package cmd

import "testing"

// The dispatcher's core: the default library denies the git-add rule, allows
// look-alikes, and records per-rule state — the same guarantees as guard, now
// served from a data library through one in-process evaluation.
func TestDispatchDefaultLibraryDeniesGitAddAll(t *testing.T) {
	lib := defaultLibrary()
	st := dispatchState{}
	dec, reason := decideDispatch(lib, st, "git add -A", false, 5, 0.5)
	if dec != "deny" {
		t.Fatalf("git add -A: decision = %q, want deny", dec)
	}
	if reason == "" || st["git-add-all"].Denied != 1 {
		t.Fatalf("expected a deny reason and recorded state, got reason=%q state=%+v", reason, st["git-add-all"])
	}
	// look-alike must pass and must not create state churn into deny
	dec, _ = decideDispatch(lib, st, "git add cmd/dispatch.go", false, 5, 0.5)
	if dec != "allow" {
		t.Fatalf("explicit-path add: decision = %q, want allow", dec)
	}
}

// A disabled rule and an unknown matcher both fail-open (never deny). This is
// the safety property: a malformed/partial library cannot block the host.
func TestDispatchFailsOpenOnDisabledAndUnknownMatcher(t *testing.T) {
	lib := ruleLibrary{Rules: []libraryRule{
		{ID: "off", Matcher: "git_add_all", Reason: "x %s", Enabled: false},
		{ID: "ghost", Matcher: "no_such_matcher", Reason: "y", Enabled: true},
	}}
	st := dispatchState{}
	if dec, _ := decideDispatch(lib, st, "git add -A", false, 5, 0.5); dec != "allow" {
		t.Fatalf("disabled rule + unknown matcher should allow, got %q", dec)
	}
	if len(st) != 0 {
		t.Fatalf("no rule should have matched, but state has %d entries", len(st))
	}
}

// Per-rule state is isolated, and each rule's override gate (the constraint
// drift signal) trips independently — the property that lets a library of many
// rules each self-monitor without a central babysitter.
func TestDispatchPerRuleOverrideGateIsIndependent(t *testing.T) {
	lib := defaultLibrary()
	st := dispatchState{}
	// 6 bypassed triggers on the one rule → its gate trips, in isolation.
	for i := 0; i < 6; i++ {
		decideDispatch(lib, st, "git add .", true, 5, 0.5)
	}
	rs := st["git-add-all"]
	if rs == nil || rs.Triggered != 6 || rs.Bypassed != 6 {
		t.Fatalf("expected 6 bypassed triggers, got %+v", rs)
	}
	if !rs.NeedsRevision {
		t.Fatal("sustained override should flag the rule NeedsRevision")
	}
	if rs.Denied != 0 {
		t.Fatalf("bypassed commands must not count as denied, got %d", rs.Denied)
	}
}

// First-deny-wins: with two matching enabled rules, the emitted reason is the
// first rule's, and BOTH rules record the trigger (so each self-monitors).
func TestDispatchFirstDenyWinsButAllRulesRecord(t *testing.T) {
	lib := ruleLibrary{Rules: []libraryRule{
		{ID: "first", Matcher: "git_add_all", Reason: "FIRST %s", Enabled: true},
		{ID: "second", Matcher: "git_add_all", Reason: "SECOND %s", Enabled: true},
	}}
	st := dispatchState{}
	dec, reason := decideDispatch(lib, st, "git add -A", false, 5, 0.5)
	if dec != "deny" || reason != "FIRST -A" {
		t.Fatalf("want deny with FIRST reason, got %q / %q", dec, reason)
	}
	if st["first"].Denied != 1 || st["second"].Denied != 1 {
		t.Fatalf("both rules should record the match, got first=%+v second=%+v", st["first"], st["second"])
	}
}
