package defnverify

import "testing"

// TestParseImpactCovered parses a real `defn impact` output with coverage.
func TestParseImpactCovered(t *testing.T) {
	out := `defn: using embedded .defn/
discover.Scan (function)
  module: github.com/justinstimatze/crystal/internal/discover

  direct callers: 3
    T TestDiscoversByStructureNotName

  tests covering this: 3
    TestDiscoversByStructureNotName
    TestNoCandidateBelowThreshold
    TestScansRealCorpus
`
	imp := parseImpact("Scan", out)
	if imp.Covered != 3 {
		t.Errorf("Covered = %d, want 3", imp.Covered)
	}
	if imp.Module != "github.com/justinstimatze/crystal/internal/discover" {
		t.Errorf("Module = %q", imp.Module)
	}
	if len(imp.Tests) != 3 {
		t.Errorf("Tests = %v, want 3", imp.Tests)
	}
}

// TestParseImpactUncovered parses the zero-coverage shape — the REFUSE case.
func TestParseImpactUncovered(t *testing.T) {
	out := `defn: using embedded .defn/
cmdspec.(CmdSpec) Commandish (method)
  module: github.com/justinstimatze/crystal/internal/cmdspec

  tests covering this: 0
    (none — this definition has no test coverage)
`
	imp := parseImpact("Commandish", out)
	if imp.Covered != 0 {
		t.Errorf("Covered = %d, want 0", imp.Covered)
	}
	if len(imp.Tests) != 0 {
		t.Errorf("Tests = %v, want none", imp.Tests)
	}
}
