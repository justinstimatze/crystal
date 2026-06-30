// Package defnverify is the defn-backed Verifier instance — the production form
// of the Verifier stage in docs/STAGE_INTERFACES.md, generalized off kong
// scaffolds onto ARBITRARY Go definitions. It shells out to the `defn` CLI
// (AI-native code database) for two things the hand-rolled kong verifier could
// not give domain-generally:
//
//	Confidence(name) — `defn impact <name>` reports the count of tests covering a
//	                   definition. That coverage IS the gate's honesty: a
//	                   definition with zero covering tests is UNVERIFIABLE, so
//	                   crystal must REFUSE to crystallize it (no-verifier-no-
//	                   crystallization) rather than sediment a silent regression.
//	Verify(name)     — run the covering tests (scoped, not the whole suite) and
//	                   pass iff green. This is "affected tests pass" — the
//	                   domain-general operational gate dogfood proved you need.
//
// Requires the target repo indexed by defn (`defn ingest <path>` → .defn/).
package defnverify

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Impact is the parsed `defn impact --json <name>` result.
type Impact struct {
	Name        string
	Module      string   // import path of the definition's package
	SourceFile  string   // repo-relative file the definition lives in (defn knows it)
	Tests       []string // covering test names
	Covered     int      // len(Tests) — the Confidence signal
	Callers     int      // direct callers — count
	BlastRadius string   // low | medium | high — defn's categorical Selector signal
}

// impactJSON mirrors `defn impact --json` (the robust interface, vs scraping text).
type impactJSON struct {
	BlastRadius string `json:"blast_radius"`
	Definition  struct {
		Name       string `json:"name"`
		SourceFile string `json:"source_file"`
	} `json:"definition"`
	DirectCallers []struct{} `json:"direct_callers"`
	Module        string     `json:"module"`
	Tests         []struct {
		Name string `json:"name"`
	} `json:"tests"`
}

// Verdict gates one definition.
type Verdict struct {
	Name       string
	Confidence int  // covering test count
	Verifiable bool // Confidence > 0 — anything else crystal must refuse
	TestsPass  bool // covering tests green (only run when Verifiable)
	Detail     string
}

// Available reports whether the `defn` CLI is reachable and the repo indexed.
func Available() error {
	if _, err := exec.LookPath("defn"); err != nil {
		return fmt.Errorf("defn not on PATH: %w", err)
	}
	out, err := run("defn", "status")
	if err != nil {
		return fmt.Errorf("defn status failed (repo not indexed? run `defn ingest .`): %v", err)
	}
	if strings.Contains(out, "not a defn") || strings.Contains(out, "no .defn") {
		return fmt.Errorf("repo not indexed by defn; run `defn ingest .`")
	}
	return nil
}

// Stats parses `defn status` for the module + definition totals.
func Stats() (modules, defs int, err error) {
	out, err := run("defn", "status")
	if err != nil {
		return 0, 0, err
	}
	// line like: "21 modules, 716 definitions"
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "modules") && strings.Contains(ln, "definitions") {
			fmt.Sscanf(strings.TrimSpace(ln), "%d modules, %d definitions", &modules, &defs)
		}
	}
	return modules, defs, nil
}

// ExportedUnits counts the crystallizable surface: exported, non-test
// definitions. This is the HONEST denominator for the verifiable fraction —
// `defn untested` counts exported defs, so the total must too (not all defs).
func ExportedUnits() (int, error) {
	out, err := run("defn", "query", "SELECT COUNT(*) AS n FROM definitions WHERE exported = TRUE AND test = FALSE")
	if err != nil {
		return 0, err
	}
	i := strings.Index(out, "[")
	if i < 0 {
		return 0, fmt.Errorf("no JSON in defn query output")
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out[i:]), &rows); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	switch v := rows[0]["n"].(type) {
	case float64:
		return int(v), nil
	case string:
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n, nil
	}
	return 0, fmt.Errorf("unexpected count shape %T", rows[0]["n"])
}

// Untested parses `defn untested` for the count of uncovered exported defs.
func Untested() (count int, err error) {
	out, err := run("defn", "untested")
	if err != nil {
		return 0, err
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.Contains(ln, "without direct test coverage") {
			fmt.Sscanf(ln, "%d exported definitions", &count)
			return count, nil
		}
	}
	return 0, nil
}

// ImpactOf runs `defn impact --json <name>` and parses the structured result.
// The --json flag must precede the name.
func ImpactOf(name string) (Impact, error) {
	out, err := run("defn", "impact", "--json", name)
	if err != nil {
		return Impact{}, err
	}
	return parseImpactJSON(name, out)
}

// parseImpactJSON maps `defn impact --json` to an Impact. Split out so it is
// CI-testable without invoking defn.
func parseImpactJSON(name, out string) (Impact, error) {
	i := strings.Index(out, "{") // skip the "defn: using embedded .defn/" preamble
	if i < 0 {
		return Impact{}, fmt.Errorf("no JSON in defn impact output for %q", name)
	}
	var j impactJSON
	if err := json.Unmarshal([]byte(out[i:]), &j); err != nil {
		return Impact{}, fmt.Errorf("decode defn impact JSON: %w", err)
	}
	imp := Impact{
		Name:        name,
		Module:      j.Module,
		SourceFile:  j.Definition.SourceFile,
		Callers:     len(j.DirectCallers),
		BlastRadius: j.BlastRadius,
	}
	for _, t := range j.Tests {
		imp.Tests = append(imp.Tests, t.Name)
	}
	imp.Covered = len(imp.Tests)
	return imp, nil
}

// Verify gates a definition: Confidence = coverage; if verifiable, run the
// covering tests (scoped to their names + package) and report pass/fail.
func Verify(name string) (Verdict, error) {
	imp, err := ImpactOf(name)
	if err != nil {
		return Verdict{}, err
	}
	v := Verdict{Name: name, Confidence: imp.Covered, Verifiable: imp.Covered > 0}
	if !v.Verifiable {
		v.Detail = "no covering tests — UNVERIFIABLE; crystal must refuse to crystallize this (would sediment a silent regression)"
		return v, nil
	}
	// Run ONLY the covering tests, scoped to the definition's package — the
	// "affected tests" defn identifies, not the whole suite.
	runExpr := "^(" + strings.Join(imp.Tests, "|") + ")$"
	pkg := imp.Module
	if pkg == "" {
		pkg = "./..."
	}
	out, terr := run("go", "test", "-run", runExpr, pkg)
	v.TestsPass = terr == nil
	if !v.TestsPass {
		v.Detail = "covering tests FAILED: " + tail(out)
	} else {
		v.Detail = fmt.Sprintf("%d covering test(s) green (scoped to %s)", imp.Covered, pkg)
	}
	return v, nil
}

func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 240 {
		return "…" + s[len(s)-240:]
	}
	return s
}
