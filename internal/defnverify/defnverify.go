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
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Impact is the parsed `defn impact <name>` result.
type Impact struct {
	Name    string
	Module  string   // import path of the definition's package
	Tests   []string // covering test names
	Covered int      // len(Tests) — the Confidence signal
	Callers int      // direct callers — the blast-radius signal (low = safer to swap)
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

// ImpactOf runs `defn impact <name>` and parses coverage + covering tests.
func ImpactOf(name string) (Impact, error) {
	out, err := run("defn", "impact", name)
	if err != nil {
		return Impact{}, err
	}
	return parseImpact(name, out), nil
}

// parseImpact extracts coverage + covering tests from `defn impact` output. Split
// out from ImpactOf so the fragile shell-output parsing is CI-testable without
// invoking defn.
func parseImpact(name, out string) Impact {
	imp := Impact{Name: name}
	sc := bufio.NewScanner(strings.NewReader(out))
	inTests := false
	for sc.Scan() {
		line := sc.Text()
		t := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(t, "module:"):
			imp.Module = strings.TrimSpace(strings.TrimPrefix(t, "module:"))
		case strings.HasPrefix(t, "direct callers:"):
			fmt.Sscanf(t, "direct callers: %d", &imp.Callers)
		case strings.HasPrefix(t, "tests covering this:"):
			fmt.Sscanf(t, "tests covering this: %d", &imp.Covered)
			inTests = true
		case inTests:
			if t == "" || strings.HasPrefix(t, "(none") || !strings.HasPrefix(t, "Test") {
				inTests = false
				continue
			}
			imp.Tests = append(imp.Tests, t)
		}
	}
	return imp
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
