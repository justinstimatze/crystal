package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// AuthorCmd is the next rung after `triage`: instead of HAND-writing the
// deterministic verifier, the expensive tier AUTHORS it, gated, and re-authors
// it when the input distribution drifts. This is the actual crystal mechanism
// (`triage`'s rules were hand-written to prove the shape).
//
// The loop, on the real Bash corpus:
//
//	author  — Opus reads a TRAIN sample of (command, reference-label) pairs and
//	          emits a deterministic rule table (token → category) as JSON. The
//	          reference labels come from the hand-authored `detClassify` — the
//	          known-good verifier `triage` already validated; the question is
//	          whether the expensive tier can author an equivalent.
//	gate    — apply the authored rules to a HELD-OUT set; promote ONLY if
//	          accuracy vs the reference ≥ threshold. No-verifier-no-crystallize:
//	          a low-fidelity authored artifact is rejected, never served. The
//	          key thesis measurement is the negative control: a deliberately
//	          CORRUPTED rule table must be REJECTED by the same gate.
//	drift   — stream a NEW command class the authored rules were never shown
//	          (containers: docker/podman/kubectl). Windowed M-in-W demotion
//	          fires on the divergences → re-author WITH the new class in train →
//	          the regenerated rules cover it and re-pass the gate.
type AuthorCmd struct {
	Corpus    string   `help:"Corpus dir of real records." default:"testdata/corpus"`
	Home      []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir  string   `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Threshold float64  `help:"Promote gate: minimum holdout accuracy vs the reference verifier." default:"0.9"`
	Sample    int      `help:"Cap on labeled examples shown to the author (deterministic subsample of train); the full holdout still gates." default:"200"`
	Model     string   `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	DriftM    int      `help:"Demote on M divergences..." default:"3"`
	DriftW    int      `help:"...within a sliding window of W streamed commands." default:"5"`
	Verbose   bool     `help:"Dump the authored rule table and per-command applied/reference/match rows."`
}

// labeledCmd pairs a command with its reference (ground-truth) category.
type labeledCmd struct{ cmd, ref string }

// authoredRule is one entry of the tier-authored deterministic classifier.
// Match is "prefix" (first token of a command segment equals Token) or
// "contains" (Token is a substring of the segment). Rules are applied
// per &&/;-segment with the same "first real action beats a leading cd" reduce
// the hand-authored verifier uses, so the authored artifact inherits the
// compound-command fix `triage` earned on real data.
type authoredRule struct {
	Match    string `json:"match"`
	Token    string `json:"token"`
	Category string `json:"category"`
}

type ruleTable struct {
	Rules []authoredRule `json:"rules"`
}

// applyRule returns the category a single rule assigns to a segment, or "".
func (r authoredRule) applyTo(seg string) string {
	fields := strings.Fields(strings.TrimSpace(seg))
	if len(fields) == 0 {
		return ""
	}
	switch r.Match {
	case "prefix":
		if fields[0] == r.Token {
			return r.Category
		}
	case "contains":
		if strings.Contains(seg, r.Token) {
			return r.Category
		}
	}
	return ""
}

// classify applies the rule table to a (possibly compound) command, mirroring
// detClassify's segment scan: the first real action beats a leading cd/nav.
func (t ruleTable) classify(cmd string) string {
	c := strings.TrimSpace(strings.ToLower(cmd))
	best := ""
	for _, seg := range splitSegments(c) {
		for _, r := range t.Rules {
			cat := r.applyTo(seg)
			if cat == "" {
				continue
			}
			if cat != "nav" {
				return cat
			}
			best = "nav"
			break // first matching rule wins for this segment
		}
	}
	return best
}

func (c *AuthorCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	cmds, src, err := loadBashCommands(c.Corpus, c.Home)
	if err != nil {
		return usageError{err}
	}
	if len(cmds) == 0 {
		return usageError{fmt.Errorf("no Bash commands found in %s", src)}
	}
	ctx := context.Background()

	// Reference labels = the hand-authored, already-validated verifier. We can
	// only GATE on commands the reference deterministically covers (the rest is
	// the residual, which has no trustworthy ground truth here).
	var covered []labeledCmd
	for _, cmd := range cmds {
		if ref := detClassify(cmd); ref != "" {
			covered = append(covered, labeledCmd{cmd, ref})
		}
	}
	if len(covered) < 8 {
		return usageError{fmt.Errorf("only %d reference-covered commands; too few to train+gate", len(covered))}
	}
	// Deterministic train/holdout split: even indices train, odd hold out. No
	// RNG (a fixed split keeps reruns cache-stable).
	var train, holdout []labeledCmd
	for i, l := range covered {
		if i%2 == 0 {
			train = append(train, l)
		} else {
			holdout = append(holdout, l)
		}
	}

	fmt.Printf("author: %d real Bash commands (%s); %d reference-covered → %d train / %d holdout\n\n",
		len(cmds), src, len(covered), len(train), len(holdout))

	// ---- AUTHOR ----
	// You author from a representative SAMPLE, not the whole corpus — the full
	// holdout still gates. Subsample deterministically (cache-stable).
	authorSet := subsample(train, c.Sample)
	if len(authorSet) < len(train) {
		fmt.Printf("(authoring from a %d-command sample of the %d-command train set)\n\n", len(authorSet), len(train))
	}
	table, ares, err := authorRules(ctx, client, c.Model, triageCategories, authorSet)
	if err != nil {
		return usageError{fmt.Errorf("authoring rules: %w", err)}
	}
	fmt.Printf("=== authored rule table (%d rules, by %s) ===\n", len(table.Rules), c.Model)
	if c.Verbose {
		fmt.Printf("  raw model output:\n%s\n\n", indentLines(ares.Text, "    "))
	}
	for _, r := range table.Rules {
		fmt.Printf("  %-9s %-14s → %s\n", r.Match, r.Token, r.Category)
	}
	fmt.Println()

	// ---- GATE (good authored rules) ----
	good := gate(table, holdout)
	printGate("authored", good, c.Threshold, c.Verbose)

	// ---- GATE (negative control: corrupted rules MUST be rejected) ----
	bad := corruptTable(table)
	badRep := gate(bad, holdout)
	fmt.Printf("\n=== negative control: corrupted rule table (categories rotated) ===\n")
	printGate("corrupted", badRep, c.Threshold, false)
	if badRep.accuracy >= c.Threshold {
		fmt.Println("  ⚠ GATE FAILED ITS JOB: a deliberately-broken rule table passed. The gate is not load-bearing.")
	} else {
		fmt.Println("  ✓ gate rejected the corrupted rules — the verifier-gate catches bad authored artifacts.")
	}

	if good.accuracy < c.Threshold {
		fmt.Printf("\nDecision: REJECT — authored rules scored %.2f < %.2f; not crystallized.\n", good.accuracy, c.Threshold)
		return nil
	}
	fmt.Printf("\nDecision: PROMOTE — authored rules scored %.2f ≥ %.2f.\n", good.accuracy, c.Threshold)

	// ---- DRIFT + RE-AUTHOR ----
	c.driftAndReauthor(ctx, client, authorSet)
	return nil
}

// subsample returns at most n elements of s, evenly spaced (deterministic).
func subsample(s []labeledCmd, n int) []labeledCmd {
	if n <= 0 || len(s) <= n {
		return s
	}
	out := make([]labeledCmd, 0, n)
	// stride so we span the whole slice, not just the head
	step := float64(len(s)) / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, s[int(float64(i)*step)])
	}
	return out
}

// gateResult is the holdout outcome of applying one rule table.
type gateResult struct {
	n, matched int
	accuracy   float64
	rows       []gateRow
}

type gateRow struct {
	cmd, applied, ref string
	ok                bool
}

// gate applies a rule table to the holdout and scores it against the
// reference labels. Producer-verifier asymmetry: scoring is pure deterministic
// comparison, far cheaper than authoring.
func gate(t ruleTable, holdout []labeledCmd) gateResult {
	res := gateResult{n: len(holdout)}
	for _, l := range holdout {
		got := t.classify(l.cmd)
		ok := got == l.ref
		if ok {
			res.matched++
		}
		res.rows = append(res.rows, gateRow{l.cmd, got, l.ref, ok})
	}
	if res.n > 0 {
		res.accuracy = float64(res.matched) / float64(res.n)
	}
	return res
}

func printGate(label string, g gateResult, thr float64, verbose bool) {
	if verbose {
		for _, r := range g.rows {
			mark := "✓"
			if !r.ok {
				mark = "✗"
			}
			a := r.applied
			if a == "" {
				a = "—(no rule)"
			}
			fmt.Printf("  %s applied=%-13s ref=%-13s %s\n", mark, a, r.ref, truncate(r.cmd, 44))
		}
	}
	fmt.Printf("  %s: %d/%d = accuracy %.2f (gate %.2f)\n", label, g.matched, g.n, g.accuracy, thr)
}

// corruptTable is the negative control: rotate every rule's category to a
// different one, producing a syntactically valid but semantically wrong table
// the gate must reject.
func corruptTable(t ruleTable) ruleTable {
	out := ruleTable{Rules: make([]authoredRule, len(t.Rules))}
	for i, r := range t.Rules {
		out.Rules[i] = authoredRule{Match: r.Match, Token: r.Token, Category: rotateCategory(r.Category)}
	}
	return out
}

func rotateCategory(cat string) string {
	for i, c := range triageCategories {
		if c == cat {
			return triageCategories[(i+1)%len(triageCategories)]
		}
	}
	return "other"
}

// authorRules asks the expensive tier to write a deterministic rule table from
// labeled examples over the given category set. Uses Complete (adaptive
// thinking) since the model benefits from reasoning over the examples;
// fail-loud on unparseable JSON (never default to an empty table that would
// silently fail the gate).
func authorRules(ctx context.Context, client *llm.Client, model string, cats []string, train []labeledCmd) (ruleTable, llm.Result, error) {
	var b strings.Builder
	b.WriteString("Here are shell commands and their correct category:\n\n")
	for _, l := range train {
		fmt.Fprintf(&b, "%s\t=> %s\n", l.cmd, l.ref)
	}
	sys := "You write a DETERMINISTIC classifier as a rule table. Categories: " +
		strings.Join(cats, ", ") + ".\n" +
		"Output ONLY JSON: {\"rules\":[{\"match\":\"prefix\"|\"contains\",\"token\":\"...\",\"category\":\"...\"}]}.\n" +
		"\"prefix\" matches when the first token of a command segment equals token (commands are split on && and ;).\n" +
		"\"contains\" matches when token is a substring of the segment. First matching rule wins per segment; " +
		"a real action beats a leading cd. Write general rules (by leading binary), not one rule per command. No prose."
	r, err := client.Complete(ctx, model, sys, b.String(), 8192)
	if err != nil {
		return ruleTable{}, r, err
	}
	t, err := parseRuleTable(r.Text, cats)
	if err != nil {
		return ruleTable{}, r, err
	}
	return t, r, nil
}

// parseRuleTable extracts the JSON object from the model's reply (tolerating a
// fenced ```json block) and validates it. Fail loud: an empty rule set or an
// unknown category is an error, not a silent pass.
func parseRuleTable(text string, cats []string) (ruleTable, error) {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(s, "json")
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	start, end := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ruleTable{}, fmt.Errorf("no JSON object in model output: %q", truncate(text, 80))
	}
	var t ruleTable
	if err := json.Unmarshal([]byte(s[start:end+1]), &t); err != nil {
		return ruleTable{}, fmt.Errorf("unmarshal rule table: %w", err)
	}
	if len(t.Rules) == 0 {
		return ruleTable{}, fmt.Errorf("authored table has zero rules")
	}
	for _, r := range t.Rules {
		if r.Match != "prefix" && r.Match != "contains" {
			return ruleTable{}, fmt.Errorf("rule has bad match kind %q", r.Match)
		}
		if !inSet(r.Category, cats) {
			return ruleTable{}, fmt.Errorf("rule targets unknown category %q", r.Category)
		}
	}
	return t, nil
}

func inSet(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

func indentLines(s, pad string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}
