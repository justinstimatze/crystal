package defnverify

import "testing"

// TestParseImpactCovered parses a real `defn impact --json` payload with coverage.
func TestParseImpactCovered(t *testing.T) {
	out := `defn: using embedded .defn/
{
  "blast_radius": "low",
  "definition": { "name": "Kebab", "kind": "function", "source_file": "internal/cmdspec/cmdspec.go" },
  "direct_callers": [ {"name":"TestKebab"}, {"name":"Verb"} ],
  "module": "github.com/justinstimatze/crystal/internal/cmdspec",
  "tests": [ {"name":"TestKebab"} ],
  "transitive_count": 3
}`
	imp, err := parseImpactJSON("Kebab", out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if imp.Covered != 1 || len(imp.Tests) != 1 || imp.Tests[0] != "TestKebab" {
		t.Errorf("coverage = %d %v", imp.Covered, imp.Tests)
	}
	if imp.BlastRadius != "low" || imp.Callers != 2 {
		t.Errorf("blast radius %q, callers %d", imp.BlastRadius, imp.Callers)
	}
	if imp.SourceFile != "internal/cmdspec/cmdspec.go" {
		t.Errorf("SourceFile = %q", imp.SourceFile)
	}
}

// TestParseImpactUncovered parses the zero-coverage shape — the REFUSE case.
func TestParseImpactUncovered(t *testing.T) {
	out := `defn: using embedded .defn/
{
  "blast_radius": "low",
  "definition": { "name": "Commandish", "source_file": "internal/cmdspec/cmdspec.go" },
  "direct_callers": [],
  "module": "github.com/justinstimatze/crystal/internal/cmdspec",
  "tests": []
}`
	imp, err := parseImpactJSON("Commandish", out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if imp.Covered != 0 || len(imp.Tests) != 0 {
		t.Errorf("expected zero coverage, got %d %v", imp.Covered, imp.Tests)
	}
}
