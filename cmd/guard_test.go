package cmd

import "testing"

// The load-bearing safety property: the guard fires on the real
// stage-everything forms and stays SILENT on look-alikes (a false-deny on the
// host's own commits is worse than missing a variant).
func TestMatchGitAddAll(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
		form string
	}{
		// sensitivity — the forms the rule exists to block
		{"git add -A", true, "-A"},
		{"git add --all", true, "--all"},
		{"git add .", true, "."},
		{"cd repo && git add -A && git commit -m x", true, "-A"},
		{"git status; git add .", true, "."},
		// specificity — look-alikes that must NOT fire
		{"git add path/to/file.go", false, ""},
		{"git add cmd/guard.go cmd/root.go", false, ""},
		{`git commit -m "add ."`, false, ""},
		{"git add -p", false, ""},
		{"git status", false, ""},
		{"go test ./...", false, ""},
		{"rg -n 'git add' .", false, ""}, // the phrase in another tool's args
	}
	for _, tc := range cases {
		got, form := matchGitAddAll(tc.cmd)
		if got != tc.want {
			t.Errorf("matchGitAddAll(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
		if got && form != tc.form {
			t.Errorf("matchGitAddAll(%q) form = %q, want %q", tc.cmd, form, tc.form)
		}
	}
}

// The constraint's self-monitoring: a sustained high override rate is the drift
// signal (the analog of the classifier hook's coverage-collapse demote). Below
// the activation floor it must stay quiet; once tripped it flags revision.
func TestOverrideGateIsTheConstraintDriftSignal(t *testing.T) {
	// 4 triggers, 3 bypasses (75% > 0.5) but below MinN=5 → must NOT trip yet.
	st := &guardState{Triggered: 4, Bypassed: 3}
	if st.overrideTripped(5, 0.5) {
		t.Fatal("override gate tripped before the activation floor (MinN)")
	}
	// 6 triggers, 1 bypass (17% < 50%) → obeyed rule, must NOT trip.
	st = &guardState{Triggered: 6, Bypassed: 1}
	if st.overrideTripped(5, 0.5) {
		t.Fatal("override gate tripped on a low override rate (rule is being obeyed)")
	}
	// 6 triggers, 4 bypasses (67% ≥ 50%) past the floor → MUST trip (rule is wrong).
	st = &guardState{Triggered: 6, Bypassed: 4}
	if !st.overrideTripped(5, 0.5) {
		t.Fatal("override gate failed to flag a rule the user keeps overriding")
	}
}
