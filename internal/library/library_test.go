package library

import "testing"

func testLib() *Library {
	return New([]Entry{
		{Name: "a", Rung: "code", Match: []string{"git", "add", "all"}, Avoid: []string{"explicit"}, MinConf: 0.6},
		{Name: "b", Rung: "recipe", Match: []string{"struct", "field"}, MinConf: 0.5},
	})
}

func TestServeOnMatch(t *testing.T) {
	l := testLib()
	d := l.Serve("git add all the things", 0)
	if !d.Served() || d.Entry.Name != "a" {
		t.Fatalf("expected serve a, got %s %v", d.Outcome, d.Entry)
	}
}

func TestCooldownAbstain(t *testing.T) {
	l := testLib()
	// A PARTIAL match (2/3 = 0.67) clears min_conf 0.6 but not the cooldown bar
	// 0.6+0.3=0.9 — so it serves once, then abstains on cooldown. (A perfect 1.0
	// match would override cooldown by design — strong fit beats the higher bar.)
	if d := l.Serve("git add stuff", 0); !d.Served() {
		t.Fatalf("partial match should serve first time, got %s (conf %.2f)", d.Outcome, d.Confidence)
	}
	d := l.Serve("git add stuff", 1) // same entry, within window → higher bar
	if d.Outcome != "abstain-cooldown" {
		t.Errorf("expected abstain-cooldown, got %s (conf %.2f thr %.2f)", d.Outcome, d.Confidence, d.Threshold)
	}
	// after the window, it serves again
	if d2 := l.Serve("git add stuff", 10); !d2.Served() {
		t.Errorf("expected serve after cooldown window, got %s", d2.Outcome)
	}
}

func TestTooMuchGuard(t *testing.T) {
	l := testLib()
	d := l.Serve("git add all but explicit", 0)
	if d.Outcome != "abstain-too-much" {
		t.Errorf("expected abstain-too-much (explicit trips avoid), got %s", d.Outcome)
	}
}

func TestLowConfAbstain(t *testing.T) {
	l := testLib()
	d := l.Serve("git something", 0) // only 'git' of 3 → 0.33 < 0.6
	if d.Outcome != "abstain-low-conf" {
		t.Errorf("expected abstain-low-conf, got %s (conf %.2f)", d.Outcome, d.Confidence)
	}
}

func TestDemotePromote(t *testing.T) {
	l := testLib()
	l.Demote("a")
	if d := l.Serve("git add all", 0); d.Served() {
		t.Errorf("demoted entry served: %s", d.Outcome)
	}
	l.Promote("a")
	if d := l.Serve("git add all", 5); !d.Served() {
		t.Errorf("re-promoted entry did not serve: %s", d.Outcome)
	}
}
