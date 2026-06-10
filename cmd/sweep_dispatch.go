package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// sweep_dispatch.go is crystal's OWN stateless serve path for a discovered
// constraint: author a data-driven regex matcher and GATE it before emitting a
// dispatch-library rule. This is the complement to --emit-stull: stull serves
// stateful loops (with a formal soundness proof); dispatch serves STANDING
// stateless constraints (block every occurrence, no terminal/E-HALT concern).
// The gate is the load-bearing part — a naked regex risks false-denies (the
// dispatch design rejects bare regexes for exactly this), so the authored pattern
// must match the bad forms AND reject a benign-command set before it can serve.

// benignCommands are always-allowed commands the authored regex must NOT match —
// the false-deny guard. The classic trap is denying `git commit -m "add ."`.
var benignCommands = []string{
	`git commit -m "add . to the list"`,
	`git commit -m "remove the -A flag"`,
	`echo "git add -A is banned"`,
	`rg "git add" docs/`,
	`git status`,
	`git add path/to/file.go`,
	`git log --oneline`,
	`# git add -A (a comment)`,
}

// authoredMatcher is the model's drafted regex plus its own labeled examples.
type authoredMatcher struct {
	Pattern   string   `json:"pattern"`
	Positives []string `json:"positives"` // bad commands the rule SHOULD deny
	Negatives []string `json:"negatives"` // benign commands it must NOT deny
}

// emitDispatchRule authors + gates a regex matcher for one constraint signature,
// then emits a dispatch-library rule (proposal). Mirrors the producer-verifier
// discipline: the expensive tier proposes, a cheap deterministic check verifies.
func (c *SweepCmd) emitDispatchRule(signature, exampleRule string) error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	fmt.Printf("crystal sweep --emit-dispatch: constraint %q → a gated dispatch regex rule\n", signature)
	fmt.Printf("  authoring a matcher with %s, then gating it against benign commands (no false-deny)...\n\n", c.Model)

	sys := "You write a single Go (RE2) regular expression that detects a specific BAD shell command form, " +
		"for a deny-rule. It must match the bad form and NOT match benign commands that merely mention it. " +
		"Reply ONLY with JSON: {\"pattern\": \"...\", \"positives\": [\"<bad cmds it must match>\"], \"negatives\": [\"<benign cmds it must NOT match>\"]}. " +
		"RE2 only (no backreferences/lookarounds). Escape backslashes for JSON."
	prompt := fmt.Sprintf("Constraint signature: %q\nA real rule line that re-encoded it across projects:\n%s\n\nWrite the matcher JSON.", signature, exampleRule)

	r, err := client.Complete(context.Background(), c.Model, sys, prompt, 800)
	if err != nil {
		return fmt.Errorf("authoring matcher: %w", err)
	}
	am, err := parseAuthoredMatcher(r.Text)
	if err != nil {
		return fmt.Errorf("model did not return usable matcher JSON: %w", err)
	}

	gate := gateDispatchRegex(am)
	fmt.Printf("=== producer-verifier gate (no-execution) ===\n")
	for _, line := range gate.report {
		fmt.Printf("  %s\n", line)
	}
	if !gate.passed {
		fmt.Printf("\n  → REJECT: the authored regex failed the false-deny / coverage gate; NOT emitted.\n")
		return nil
	}

	rule := libraryRule{
		ID:      "deny-" + strings.NewReplacer("<", "", ">", "", " ", "-").Replace(signature),
		Matcher: "regex",
		Pattern: am.Pattern,
		Reason:  fmt.Sprintf("crystal: %%s trips a crystallized constraint (%q). Use the prescribed form. Override one call with CRYSTAL_GUARD_SKIP=1.", signature),
		Enabled: true,
	}
	outDir := filepath.Clean(filepath.Join(c.CacheDir, "..", ".crystal-proposals"))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return usageError{err}
	}
	path := filepath.Join(outDir, "dispatch-"+rule.ID+".json")
	b, _ := json.MarshalIndent(ruleLibrary{Rules: []libraryRule{rule}}, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return usageError{err}
	}
	fmt.Printf("\n  → PROMOTE (proposal only): gate passed; wrote dispatch rule to %s\n", path)
	fmt.Printf("  Serve it with `crystal dispatch --rules %s` after review. (Cached: %v)\n", path, r.Cached)
	return nil
}

// gateDispatchRegex verifies the authored regex WITHOUT serving it:
//  1. it compiles (RE2).
//  2. coverage — it matches every positive the model labeled.
//  3. false-deny guard — it matches NONE of the model's negatives NOR the
//     standing benignCommands set (the load-bearing check the dispatch design demands).
func gateDispatchRegex(am authoredMatcher) authorGate {
	var g authorGate
	pass := true

	re, err := regexp.Compile(am.Pattern)
	if err != nil {
		g.report = append(g.report, "✗ compile: "+err.Error())
		g.passed = false
		return g
	}
	g.report = append(g.report, "✓ compiles (RE2): "+am.Pattern)

	var missed []string
	for _, p := range am.Positives {
		if !re.MatchString(p) {
			missed = append(missed, p)
		}
	}
	if len(missed) > 0 {
		g.report = append(g.report, "✗ coverage: regex misses bad forms it should catch: "+strings.Join(missed, " | "))
		pass = false
	} else if len(am.Positives) > 0 {
		g.report = append(g.report, fmt.Sprintf("✓ coverage: matches all %d labeled bad forms", len(am.Positives)))
	}

	var falseDenies []string
	for _, b := range append(append([]string{}, am.Negatives...), benignCommands...) {
		if re.MatchString(b) {
			falseDenies = append(falseDenies, b)
		}
	}
	if len(falseDenies) > 0 {
		g.report = append(g.report, "✗ false-deny guard: regex matches BENIGN commands: "+strings.Join(dedupeStrings(falseDenies), " | "))
		pass = false
	} else {
		g.report = append(g.report, fmt.Sprintf("✓ false-deny guard: matches none of %d benign commands", len(am.Negatives)+len(benignCommands)))
	}

	g.passed = pass
	return g
}

// parseAuthoredMatcher extracts the matcher JSON from a model reply (tolerating
// a code fence or surrounding prose).
func parseAuthoredMatcher(text string) (authoredMatcher, error) {
	s := stripFences(strings.TrimSpace(text))
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndexByte(s, '}'); j >= 0 {
		s = s[:j+1]
	}
	var am authoredMatcher
	if err := json.Unmarshal([]byte(s), &am); err != nil {
		return authoredMatcher{}, err
	}
	if am.Pattern == "" {
		return authoredMatcher{}, fmt.Errorf("empty pattern")
	}
	return am, nil
}
