package library

import "testing"

// a well-formed entry: specific trigger, a working avoid guard.
func goodEntry() Entry {
	return Entry{
		Name:     "guard-git-add-all",
		Rung:     "code",
		Match:    []string{"git", "add", "-a", "--all"},
		Avoid:    []string{"explicit", "intentional"},
		Artifact: "deny — stage explicit paths instead",
		MinConf:  0.5,
	}
}

func TestGateEntryPasses(t *testing.T) {
	g := GateEntry(goodEntry(),
		[]string{"please git add -A and commit", "run git add --all now"}, // positives
		[]string{"git add --all but it is explicit this once"},            // too-much
		nil)
	if !g.Passed {
		t.Fatalf("good entry should pass; report:\n%v", g.Report)
	}
}

func TestGateRejectsNoCoverage(t *testing.T) {
	// MinConf too high to ever fire on its own evidence.
	e := goodEntry()
	e.MinConf = 0.99
	g := GateEntry(e, []string{"please git add -A"}, nil, nil)
	if g.Passed {
		t.Fatalf("entry that can't serve on its evidence must fail coverage")
	}
}

func TestGateRejectsFalseServe(t *testing.T) {
	// A match list so broad a benign context (running tests) trips it.
	e := goodEntry()
	e.Match = []string{"run", "the"}
	e.Avoid = nil
	e.MinConf = 0.5
	g := GateEntry(e, []string{"run the thing"}, nil, nil)
	if g.Passed {
		t.Fatalf("entry firing on benign contexts must fail false-serve guard")
	}
}

func TestGateRejectsDeadAvoidGuard(t *testing.T) {
	// A too-much context is given but the avoid token never appears in it.
	e := goodEntry()
	e.Avoid = []string{"nonexistenttoken"}
	g := GateEntry(e,
		[]string{"please git add -A"},
		[]string{"git add --all and it is explicit"}, // 'explicit', not the avoid token
		nil)
	if g.Passed {
		t.Fatalf("entry whose avoid guard fails to trip must be rejected")
	}
}

func TestGateRejectsNoMatchTokens(t *testing.T) {
	e := goodEntry()
	e.Match = nil
	if GateEntry(e, nil, nil, nil).Passed {
		t.Fatalf("entry with no trigger must fail")
	}
}
