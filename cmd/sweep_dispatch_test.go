package cmd

import "testing"

// TestDispatchRegexGateHasTeeth: the gate must reject a regex that false-denies a
// benign command, misses a labeled bad form, or doesn't compile — the false-deny
// risk the dispatch design rejects naked regexes for.
func TestDispatchRegexGateHasTeeth(t *testing.T) {
	// Good: anchored to command-start so it matches the bad form but NOT the same
	// text mentioned inside a commit message / echo / comment (the false-deny trap
	// a naive \bgit add -A\b substring falls into — which the gate correctly catches).
	good := authoredMatcher{
		Pattern:   `^git add -A\b`,
		Positives: []string{"git add -A", "git add -A && git commit"},
		Negatives: []string{`git commit -m "git add -A"`},
	}
	if g := gateDispatchRegex(good); !g.passed {
		t.Errorf("good (anchored) matcher rejected: %v", g.report)
	}

	// False-deny: an over-broad pattern that also hits the benign-command set.
	broad := authoredMatcher{Pattern: `git add`, Positives: []string{"git add -A"}}
	if g := gateDispatchRegex(broad); g.passed {
		t.Error("over-broad `git add` should be REJECTED (false-denies `git add path/to/file.go`)")
	}

	// Missing coverage: doesn't match a labeled positive.
	miss := authoredMatcher{Pattern: `git add --all`, Positives: []string{"git add -A"}}
	if g := gateDispatchRegex(miss); g.passed {
		t.Error("a pattern that misses a labeled bad form should be REJECTED")
	}

	// Bad compile.
	bad := authoredMatcher{Pattern: `git add (-A`, Positives: []string{"git add -A"}}
	if g := gateDispatchRegex(bad); g.passed {
		t.Error("an uncompilable regex should be REJECTED")
	}
}

// TestResolveRegexMatcher confirms the data-driven regex matcher serves from the
// rule's Pattern, and a bad pattern fails open (not-ok), never denying.
func TestResolveRegexMatcher(t *testing.T) {
	r := libraryRule{Matcher: "regex", Pattern: `\bgit\s+add\s+-A\b`}
	m, ok := resolveMatcher(r)
	if !ok {
		t.Fatal("regex matcher should resolve")
	}
	if hit, _ := m("git add -A"); !hit {
		t.Error("should match the bad form")
	}
	if hit, _ := m("git add file.go"); hit {
		t.Error("should NOT match a benign add")
	}
	if _, ok := resolveMatcher(libraryRule{Matcher: "regex", Pattern: "("}); ok {
		t.Error("an uncompilable pattern must fail open (not-ok), never deny")
	}
}
