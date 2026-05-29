package cmd

import "testing"

func TestParseRuleTable(t *testing.T) {
	ok := `{"rules":[{"match":"prefix","token":"git","category":"git"},{"match":"contains","token":"test","category":"build/test"}]}`
	tbl, err := parseRuleTable(ok, triageCategories)
	if err != nil {
		t.Fatalf("valid table rejected: %v", err)
	}
	if len(tbl.Rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(tbl.Rules))
	}
	// Fenced + prose-wrapped output must still parse.
	fenced := "Here you go:\n```json\n" + ok + "\n```\n"
	if _, err := parseRuleTable(fenced, triageCategories); err != nil {
		t.Errorf("fenced table rejected: %v", err)
	}
	// Fail-loud cases — none may silently yield an empty table.
	for _, bad := range []string{
		`{"rules":[]}`,                                                       // zero rules
		`{"rules":[{"match":"regex","token":"x","category":"git"}]}`,         // bad match kind
		`{"rules":[{"match":"prefix","token":"x","category":"frobnicate"}]}`, // unknown category
		`no json here`,
	} {
		if _, err := parseRuleTable(bad, triageCategories); err == nil {
			t.Errorf("bad table accepted: %q", bad)
		}
	}
}

func TestClassifyCompound(t *testing.T) {
	tbl := ruleTable{Rules: []authoredRule{
		{"prefix", "cd", "nav"},
		{"prefix", "git", "git"},
		{"prefix", "rg", "search/inspect"},
	}}
	cases := map[string]string{
		"git status":               "git",
		"cd src && git commit -m x": "git",            // real action beats leading cd
		"cd src && rg foo":          "search/inspect", // ditto
		"cd src":                    "nav",            // only nav present
		"unknownbin --flag":         "",               // residual
	}
	for cmd, want := range cases {
		if got := tbl.classify(cmd); got != want {
			t.Errorf("classify(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestGateAndCorrupt(t *testing.T) {
	tbl := ruleTable{Rules: []authoredRule{
		{"prefix", "git", "git"},
		{"prefix", "rg", "search/inspect"},
	}}
	holdout := []labeledCmd{
		{"git push", "git"},
		{"rg foo", "search/inspect"},
	}
	good := gate(tbl, holdout)
	if good.accuracy != 1.0 {
		t.Fatalf("good table accuracy = %.2f, want 1.0", good.accuracy)
	}
	// Corrupting every category must drop accuracy to 0 here (rotation moves
	// each label to a different one), so the gate would reject it.
	bad := corruptTable(tbl)
	if br := gate(bad, holdout); br.accuracy != 0.0 {
		t.Errorf("corrupted table accuracy = %.2f, want 0.0 (gate must reject)", br.accuracy)
	}
}

func TestContainerRefAndStreamDemote(t *testing.T) {
	if containerRef("docker build .") != "container" {
		t.Fatal("containerRef missed docker")
	}
	if containerRef("git status") != "" {
		t.Fatal("containerRef false-positive on git")
	}
	// A table that produces no "container" diverges on every drift command, so
	// a 3-in-5 window must demote within the first few.
	tbl := ruleTable{Rules: []authoredRule{{"prefix", "git", "git"}}}
	at, leaked := streamDemote(tbl, driftCommands, containerRef, 3, 5)
	if at < 0 {
		t.Fatalf("never demoted on an all-divergence stream (leaked %d)", leaked)
	}
	if at != 2 { // 3rd divergence (index 2) fills the 3-in-5 window
		t.Errorf("demoted at index %d, want 2", at)
	}
}
