package discover

import (
	"testing"

	"github.com/justinstimatze/crystal/internal/cmdspec"
)

func helped(name string, n int) cmdspec.CmdSpec {
	s := cmdspec.CmdSpec{Name: name}
	for i := 0; i < n; i++ {
		s.Fields = append(s.Fields, cmdspec.FlagSpec{Name: "F", Type: "string", Help: "h"})
	}
	return s
}

func bare(name string, n int) cmdspec.CmdSpec {
	s := cmdspec.CmdSpec{Name: name}
	for i := 0; i < n; i++ {
		s.Fields = append(s.Fields, cmdspec.FlagSpec{Name: "F", Type: "string"})
	}
	return s
}

// TestDiscoversByStructureNotName is the honesty claim: discovery finds the
// command shape by its help-tagged fields, so a help-tagged struct NOT named
// "*Cmd" is included and a bare-field helper that IS named "*Cmd" is excluded.
func TestDiscoversByStructureNotName(t *testing.T) {
	all := []cmdspec.CmdSpec{
		helped("Foo", 2),        // command-shaped, no "Cmd" suffix → must be found
		helped("RunThing", 3),   // command-shaped, no suffix
		helped("ProbeCmd", 2),   // command-shaped, has suffix
		helped("ServeCmd", 2),   //
		helped("ExtractCmd", 2), //
		bare("LabeledCmd", 2),   // bare fields despite "Cmd" suffix → must be rejected
		bare("ruleTable", 1),    // helper
	}
	rep := Scan(all, 5)
	if !rep.Candidate {
		t.Fatalf("expected a candidate at recurrence %d (min 5)", rep.Dominant.Recurrence())
	}
	in := map[string]bool{}
	for _, m := range rep.Dominant.Members {
		in[m.Name] = true
	}
	if !in["Foo"] || !in["RunThing"] {
		t.Error("discovery missed a help-tagged command not named *Cmd (relied on the name)")
	}
	if in["LabeledCmd"] {
		t.Error("discovery included a bare-field helper named *Cmd (matched on the name)")
	}
	if rep.Dominant.Recurrence() != 5 {
		t.Errorf("expected 5 command-shaped units, got %d", rep.Dominant.Recurrence())
	}
}

func TestNoCandidateBelowThreshold(t *testing.T) {
	rep := Scan([]cmdspec.CmdSpec{helped("A", 2), helped("B", 2)}, 5)
	if rep.Candidate {
		t.Error("2 commands should not clear a recurrence floor of 5")
	}
}

func TestScansRealCorpus(t *testing.T) {
	all, err := cmdspec.ParseAll("../../cmd")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rep := Scan(all, 5)
	if !rep.Candidate {
		t.Fatal("the real cmd/ corpus should yield a crystallization candidate")
	}
	reg, enum, slice := rep.Dominant.Breakdown()
	if enum == 0 || slice == 0 {
		t.Errorf("expected the dominant shape to include enum and slice variety, got %d/%d/%d", reg, enum, slice)
	}
}
