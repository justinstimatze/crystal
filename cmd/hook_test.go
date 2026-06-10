package cmd

import "testing"

// decideHook is the pure core of the live PreToolUse hook. These tests assert
// the behaviors the live hook must get right: it SERVES covered commands
// (injecting the deterministic category, 0 model calls) and stays silent on the
// residual; it DEMOTES on coverage collapse (both the sudden-burst gate and the
// sustained-rate gate); and — the seam wired this session — it can be
// RE-PROMOTED after a re-author swaps in a covering rule table, and it serves
// from that artifact's classifier.

// testCfg is the default drift config used in tests (mirrors HookCmd defaults).
func testCfg(classify func(string) string) driftCfg {
	return driftCfg{classify: classify, m: 3, w: 5, longW: 20, longMinN: 12, longRate: 0.35}
}

func TestDecideHookServesAndDefers(t *testing.T) {
	st := &hookState{DemotedAtTotal: -1}
	cfg := testCfg(detClassify)

	// A covered command → served with its category injected.
	d := decideHook(st, "git status", cfg)
	if !d.served || d.category != "git" {
		t.Fatalf("covered command: want served=git, got served=%v cat=%q", d.served, d.category)
	}
	if d.additionalContext == "" {
		t.Fatal("served command must inject additionalContext")
	}

	// A single residual command → silent defer, no demotion (window not collapsed).
	d = decideHook(st, "xyzzy --frobnicate", cfg)
	if d.served || d.demotedNow || d.additionalContext != "" {
		t.Fatalf("residual command: want silent defer, got %+v", d)
	}
	if st.Served != 1 || st.Deferred != 1 {
		t.Fatalf("tallies: want served=1 deferred=1, got served=%d deferred=%d", st.Served, st.Deferred)
	}
}

func TestDecideHookDemotesOnCoverageCollapse(t *testing.T) {
	st := &hookState{DemotedAtTotal: -1}
	cfg := testCfg(detClassify)
	// Three covered commands keep coverage healthy (no demote at M=3, W=5).
	for _, c := range []string{"git add .", "go test ./...", "rg foo"} {
		if d := decideHook(st, c, cfg); !d.served {
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
		d := decideHook(st, c, cfg)
		if d.demotedNow {
			demotedAt = i
			break
		}
	}
	if demotedAt != 2 {
		t.Fatalf("want demotion on the 3rd uncovered command (index 2 of the burst), got %d", demotedAt)
	}
	if !st.Demoted || st.DemotedAtTotal != 6 || st.DemoteReason != "burst" {
		t.Fatalf("want Demoted=true at total=6 via burst, got Demoted=%v total=%d reason=%q", st.Demoted, st.DemotedAtTotal, st.DemoteReason)
	}
	if !st.NeedsReauthor {
		t.Fatal("demotion must set NeedsReauthor so the re-author loop can pick it up")
	}
	if len(st.RecentUncovered) == 0 {
		t.Fatal("demotion must have captured drifted commands for the re-author")
	}
	// After demotion, every further command is a silent defer (chore on the model).
	d := decideHook(st, "git commit -m x", cfg) // covered, but tier is demoted
	if d.served || d.additionalContext != "" {
		t.Fatalf("post-demotion must defer silently even on a covered command, got %+v", d)
	}
}

// TestSustainedInterleaveEvadesBurstButCaughtByCumulative is the panel's
// 2-in-5-interleave evasion, fixed. A steady covered,drift,covered,drift,drift…
// at ~40% uncovered never reaches 3-in-5, so the burst-only gate is blind to it
// forever — but the cumulative rate gate catches it. We prove BOTH directions.
func TestSustainedInterleaveEvadesBurstButCaughtByCumulative(t *testing.T) {
	// A repeating period that holds the SHORT window (W=5) at ≤2 uncovered but
	// the long-window rate at 40%: pattern C C C U U (3 covered, 2 uncovered).
	pattern := []string{"git status", "go build ./...", "rg foo", "docker ps", "kubectl get pods"}
	stream := func() []string {
		var s []string
		for i := 0; i < 8; i++ { // 40 commands
			s = append(s, pattern...)
		}
		return s
	}

	// Burst-only config (cumulative disabled): the evasion succeeds — never demotes.
	burstOnly := driftCfg{classify: detClassify, m: 3, w: 5, longW: 0}
	st1 := &hookState{DemotedAtTotal: -1}
	for _, c := range stream() {
		if d := decideHook(st1, c, burstOnly); d.demotedNow {
			t.Fatalf("burst-only gate should be BLIND to the 2-in-5 interleave, but it demoted on %q", c)
		}
	}
	if st1.Demoted {
		t.Fatal("burst-only gate must not demote on sustained interleave (proves the evasion is real)")
	}

	// Full config (cumulative enabled): the same stream IS caught.
	full := testCfg(detClassify)
	st2 := &hookState{DemotedAtTotal: -1}
	demoted := false
	for _, c := range stream() {
		if d := decideHook(st2, c, full); d.demotedNow {
			demoted = true
			if st2.DemoteReason != "sustained" {
				t.Fatalf("interleave should trip the SUSTAINED gate, got reason %q", st2.DemoteReason)
			}
			break
		}
	}
	if !demoted {
		t.Fatal("cumulative gate must catch the sustained interleave the burst gate missed")
	}
}

// TestRepromoteRecoversFromTerminalDemotion is the panel's terminal-DoS fix: a
// demoted tier, after a re-author swaps in a rule table that covers the drifted
// class, is re-promoted and serves the previously-uncovered commands again.
func TestRepromoteRecoversFromTerminalDemotion(t *testing.T) {
	st := &hookState{DemotedAtTotal: -1}
	cfg := testCfg(detClassify)
	// Demote via a container burst.
	for _, c := range driftCommands {
		if decideHook(st, c, cfg).demotedNow {
			break
		}
	}
	if !st.Demoted {
		t.Fatal("setup: expected demotion on the container burst")
	}
	// While demoted, even a coverable command is deferred (the DoS).
	if d := decideHook(st, "docker ps -a", cfg); d.served {
		t.Fatal("demoted tier must not serve")
	}

	// The re-author loop produces a table that NOW covers containers, swaps it in,
	// and re-promotes. Serve from the new table.
	covering := ruleTable{Rules: []authoredRule{
		{Match: "prefix", Token: "docker", Category: "container"},
		{Match: "prefix", Token: "kubectl", Category: "container"},
		{Match: "prefix", Token: "podman", Category: "container"},
	}}
	repromote(st)
	if st.Demoted || st.NeedsReauthor || st.Reauthors != 1 {
		t.Fatalf("repromote must clear demotion and count the re-author, got %+v", st)
	}
	cfg2 := testCfg(covering.classify)
	d := decideHook(st, "docker build -t app .", cfg2)
	if !d.served || d.category != "container" {
		t.Fatalf("after re-promote+swap the tier must serve the drifted class, got served=%v cat=%q", d.served, d.category)
	}
}

// TestServingClassifierFromArtifact confirms the hook serves from an authored
// rule-table artifact when one is present, and falls back to detClassify when
// absent or empty — the indirection the re-author loop swaps.
func TestServingClassifierFromArtifact(t *testing.T) {
	// No path → baseline.
	f, err := servingClassifier("")
	if err != nil || f("git status") != "git" {
		t.Fatalf("empty path must serve the detClassify baseline, err=%v", err)
	}
	// Missing file → baseline (not an error: just not authored yet).
	f, err = servingClassifier("/nonexistent/rules.json")
	if err != nil || f("git status") != "git" {
		t.Fatalf("missing artifact must fall back to baseline, err=%v", err)
	}
	// A real artifact → serve from it.
	dir := t.TempDir()
	path := dir + "/rules.json"
	if err := writeRuleArtifact(path, ruleTable{Rules: []authoredRule{{Match: "prefix", Token: "docker", Category: "container"}}}); err != nil {
		t.Fatal(err)
	}
	f, err = servingClassifier(path)
	if err != nil {
		t.Fatalf("valid artifact: %v", err)
	}
	if f("docker ps") != "container" {
		t.Fatalf("artifact classifier must categorize docker as container, got %q", f("docker ps"))
	}
}
