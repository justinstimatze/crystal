package cmdspec

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestParseKongGroup(t *testing.T) {
	kv := parseKongGroup("enum='get,post,put',help='HTTP method',required")
	if kv["enum"] != "get,post,put" {
		t.Errorf("enum = %q, want %q (comma-inside-quotes must survive)", kv["enum"], "get,post,put")
	}
	if kv["help"] != "HTTP method" {
		t.Errorf("help = %q, want %q", kv["help"], "HTTP method")
	}
	if _, ok := kv["required"]; !ok {
		t.Error("bare directive 'required' should be present with empty value")
	}
}

// TestGroupedDialectEnum guards the false-negative planshift surfaced: a scaffold
// that is contract-correct in kong's grouped tag dialect must read as having the
// enum, not as a dropped one.
func TestGroupedDialectEnum(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\ntype RouteCmd struct {\n" +
		"\tPath   string `kong:\"help='Route path'\"`\n" +
		"\tMethod string `kong:\"enum='get,post,put,delete',help='HTTP method'\"`\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(dir, "route.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	specs, err := ParseAll(dir)
	if err != nil || len(specs) != 1 {
		t.Fatalf("ParseAll: %v (%d specs)", err, len(specs))
	}
	if !specs[0].HasEnum() {
		t.Error("grouped-dialect enum flag read as no-enum (the false negative)")
	}
	var method FlagSpec
	for _, f := range specs[0].Fields {
		if f.Name == "Method" {
			method = f
		}
	}
	if method.Enum != "get,post,put,delete" {
		t.Errorf("Method.Enum = %q, want the full comma list", method.Enum)
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
