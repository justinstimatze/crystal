package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// sweep_author.go closes discovery -> crystallization for PROCEDURES: the
// expensive tier drafts a shell script for a detected ceremony, and a NO-RUN
// structural gate verifies it before it's emitted. The gate is the point: you
// can't run-and-check a side-effectful ceremony (git reset --hard, gh api), so the
// verifier checks the generated artifact against the OBSERVED procedure cheaply,
// without executing it — the producer-verifier discipline applied to generated code.

// authorProcedure drafts and gates a script for one detected procedure.
func (c *SweepCmd) authorProcedure(steps, examples []string) error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	fmt.Printf("crystal sweep --author: crystallizing the top procedure\n")
	fmt.Printf("  procedure (%d steps): %s\n", len(steps), strings.Join(steps, " → "))
	fmt.Printf("  authoring a draft script with %s, then gating it (no-run structural check)...\n\n", c.Model)

	sys := "You convert a recurring shell ceremony into one clean, reusable POSIX shell script. " +
		"Rules: use `set -euo pipefail`; parameterize the volatile parts (commit messages, paths, refs) " +
		"as positional args or variables with sane defaults; keep the SAME commands the ceremony uses and " +
		"add NO new commands beyond those and trivial glue (echo, set, cd). Output ONLY the script, no prose, no code fences."
	prompt := fmt.Sprintf("The recurring ceremony, as an ordered sequence of command signatures:\n  %s\n\nReal example instances from my shell history:\n\n%s\n\nWrite the script.",
		strings.Join(steps, " -> "), strings.Join(examples, "\n---\n"))

	r, err := client.Complete(context.Background(), c.Model, sys, prompt, 2000)
	if err != nil {
		return fmt.Errorf("authoring: %w", err)
	}
	script := stripFences(r.Text)

	gate := gateAuthoredScript(script, steps)
	fmt.Printf("=== no-run gate ===\n")
	for _, line := range gate.report {
		fmt.Printf("  %s\n", line)
	}
	if !gate.passed {
		fmt.Printf("\n  → REJECT: the draft did not pass the structural gate; NOT emitted (the producer-verifier guard held).\n")
		return nil
	}

	outDir := filepath.Join(c.CacheDir, "..", ".crystal-proposals")
	outDir = filepath.Clean(outDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return usageError{err}
	}
	name := "proc-" + strings.ReplaceAll(strings.Join(leadsOf(steps), "-"), " ", "") + ".sh"
	path := filepath.Join(outDir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		return usageError{err}
	}
	fmt.Printf("\n  → PROMOTE (proposal only): gate passed; wrote draft to %s\n", path)
	fmt.Printf("  Review it before use — it is a PROPOSAL, not installed or run. (Cached: %v)\n", r.Cached)
	return nil
}

// authorGate is the no-run gate's verdict plus a human-readable report.
type authorGate struct {
	passed bool
	report []string
}

// gateAuthoredScript verifies a drafted script WITHOUT running it:
//  1. `bash -n` — it parses (syntactically valid).
//  2. hallucination guard — every command the script invokes uses a lead the
//     ceremony actually used (so it can't introduce rm/curl/ssh the ceremony
//     never had; the dangerous leads are in commandLeads, so this catches them).
//  3. fidelity — the ceremony's steps appear in the script in order (subsequence).
func gateAuthoredScript(script string, proc []string) authorGate {
	var g authorGate
	pass := true

	if err := bashSyntaxOK(script); err != nil {
		g.report = append(g.report, "✗ bash -n: "+err.Error())
		pass = false
	} else {
		g.report = append(g.report, "✓ bash -n: parses")
	}

	scriptSigs := scriptSignatures(script)
	observed := map[string]bool{}
	for _, l := range leadsOf(proc) {
		observed[l] = true
	}
	var rogue []string
	for _, s := range scriptSigs {
		lead := strings.Fields(s)[0]
		if !observed[lead] {
			rogue = append(rogue, s)
		}
	}
	if len(rogue) > 0 {
		g.report = append(g.report, "✗ hallucination guard: script invokes commands the ceremony never used: "+strings.Join(dedupeStrings(rogue), ", "))
		pass = false
	} else {
		g.report = append(g.report, "✓ hallucination guard: no command outside the observed ceremony leads")
	}

	if isOrderedSubsequence(proc, scriptSigs) {
		g.report = append(g.report, "✓ fidelity: the ceremony's steps appear in order")
	} else {
		g.report = append(g.report, fmt.Sprintf("✗ fidelity: ceremony %v is not an in-order subsequence of the script's commands %v", proc, scriptSigs))
		pass = false
	}

	g.passed = pass
	return g
}

// bashSyntaxOK runs `bash -n` (parse only, NO execution) over the script.
func bashSyntaxOK(script string) error {
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// scriptSignatures extracts the ordered command signatures a script invokes,
// skipping comments and reusing the same splitShell/procSignature the detector uses.
func scriptSignatures(script string) []string {
	var sigs []string
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, sub := range splitShell(line) {
			if sig := procSignature(stripShellPrefix(sub)); sig != "" {
				sigs = append(sigs, sig)
			}
		}
	}
	return sigs
}

// stripShellPrefix removes leading shell noise (variable assignments, `then`,
// `do`, `"$@"` wrappers) so the leading real command is what procSignature sees.
func stripShellPrefix(s string) string {
	s = strings.TrimSpace(s)
	for _, kw := range []string{"then ", "do ", "else "} {
		s = strings.TrimPrefix(s, kw)
	}
	fields := strings.Fields(s)
	i := 0
	for i < len(fields) && strings.Contains(fields[i], "=") && !commandLeads[strings.ToLower(fields[i])] {
		i++ // skip VAR=val prefixes
	}
	return strings.Join(fields[i:], " ")
}

// isOrderedSubsequence reports whether want appears in have in order (gaps ok).
func isOrderedSubsequence(want, have []string) bool {
	i := 0
	for _, h := range have {
		if i < len(want) && h == want[i] {
			i++
		}
	}
	return i == len(want)
}

// leadsOf returns the command leads (first word) of a signature list.
func leadsOf(sigs []string) []string {
	var out []string
	for _, s := range sigs {
		if f := strings.Fields(s); len(f) > 0 {
			out = append(out, f[0])
		}
	}
	return out
}

// stripFences removes a leading/trailing markdown code fence if the model added one.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 {
		lines = lines[1:] // drop opening ``` (or ```bash)
	}
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "```" {
		lines = lines[:n-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
