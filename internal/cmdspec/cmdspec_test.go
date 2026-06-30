package cmdspec

import "testing"

func TestParseCrystalCorpus(t *testing.T) {
	specs, err := Parse("../../cmd")
	if err != nil {
		t.Fatalf("parse cmd/: %v", err)
	}
	if len(specs) < 20 {
		t.Fatalf("expected ≥20 subcommands, got %d", len(specs))
	}
	byName := map[string]CmdSpec{}
	for _, s := range specs {
		byName[s.Name] = s
		if s.Name[0] < 'A' || s.Name[0] > 'Z' {
			t.Errorf("unexported struct leaked into corpus: %q", s.Name)
		}
	}
	// labeledCmd is an unexported helper, not a command — must be excluded.
	if _, ok := byName["labeledCmd"]; ok {
		t.Error("labeledCmd (unexported helper) was parsed as a command")
	}
	// HookLoopCmd carries the irregular shapes the harness leans on.
	hl, ok := byName["HookLoopCmd"]
	if !ok {
		t.Fatal("HookLoopCmd not found")
	}
	if !hl.HasEnum() {
		t.Error("HookLoopCmd should have an enum flag (Oracle)")
	}
	if !hl.HasSlice() {
		t.Error("HookLoopCmd should have a slice flag (Home)")
	}
}

func TestKebab(t *testing.T) {
	cases := map[string]string{
		"Probe":       "probe",
		"CacheDir":    "cache-dir",
		"BigProvider": "big-provider",
		"Oracle":      "oracle",
	}
	for in, want := range cases {
		if got := Kebab(in); got != want {
			t.Errorf("Kebab(%q) = %q, want %q", in, got, want)
		}
	}
}
