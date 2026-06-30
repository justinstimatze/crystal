package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinstimatze/crystal/internal/library"
	"github.com/justinstimatze/crystal/internal/llm"
)

// sweep_library.go closes the watch→author half of the SERVE tier: a discovered
// recurrence (a constraint re-encoded across N projects) is authored into a
// candidate library.Entry — the recipe artifact PLUS its groupchat-style
// triggering metadata — then GATED behaviorally before it can join the library.
//
// This is the serve-layer twin of --emit-dispatch. Where --emit-dispatch authors
// a stateless deny REGEX (one matcher, gated against benign commands), this
// authors a full SERVE entry (a recipe + match/avoid/deploy-when) and gates its
// TRIGGERING with library.GateEntry: serve on its own evidence, abstain-too-much
// where it should hold back, never serve on benign contexts. Same producer-
// verifier discipline; the model proposes, a deterministic check verifies; a
// passing entry is PROPOSED, never auto-added to the live library.

// authoredEntry is the model's drafted entry plus its own labeled gate contexts.
type authoredEntry struct {
	library.Entry
	Positives       []string `json:"positives"`         // contexts where this SHOULD serve
	TooMuchExamples []string `json:"too_much_examples"` // contexts where the Avoid guard should trip
}

// emitLibraryEntry authors + gates a serve-library entry for one constraint
// signature, then emits it as a proposal.
func (c *SweepCmd) emitLibraryEntry(signature, exampleRule string, evidence []string) error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	fmt.Printf("crystal sweep --emit-library: constraint %q → a gated serve-library entry\n", signature)
	fmt.Printf("  authoring an entry (recipe + match/avoid/deploy-when) with %s, then gating its triggering...\n\n", c.Model)

	sys := "You author one entry for a SERVE-FROM-LIBRARY system (like a meme library): a small reusable " +
		"artifact plus the metadata that decides WHEN to serve it. The serve decision is a deterministic " +
		"EXACT-TOKEN keyword match (lowercased, split on whitespace) — there is no stemming, so a `match` " +
		"token only fires when that exact word appears. Reply ONLY with JSON: " +
		"{\"name\":\"<kebab-id>\",\"rung\":\"recipe\",\"deploy_when\":\"<when it applies>\"," +
		"\"too_much_if\":\"<when NOT to serve even on a match>\",\"mechanism\":\"<why it works>\"," +
		"\"key\":\"<fingerprint vs similar entries>\",\"match\":[\"<exact lowercase trigger tokens>\"]," +
		"\"avoid\":[\"<exact lowercase too-much guard tokens>\"],\"artifact\":\"<the recipe/rule body the cheap tier serves>\"," +
		"\"min_conf\":0.5,\"positives\":[\"<2-4 REALISTIC literal user/assistant inputs where this SHOULD serve>\"]," +
		"\"too_much_examples\":[\"<1-2 realistic inputs that contain the trigger AND an avoid token, so it ABSTAINS>\"]}. " +
		"Rules that keep the gate honest: (a) positives must be literal text a user/assistant would actually type " +
		"(e.g. 'please git add -A and commit'), NOT descriptions of the situation; each positive MUST contain enough " +
		"`match` tokens to clear min_conf. (b) each too_much_example MUST contain at least one `avoid` token verbatim. " +
		"(c) `match` must be specific enough that routine unrelated requests (listing files, running tests, explaining " +
		"code) do NOT trigger it. (d) avoid tokens are exact words — use the base form that actually appears " +
		"('explicit' won't match 'explicitly'; pick the form your too_much_examples use). (e) avoid tokens must " +
		"signal a DELIBERATE HUMAN OVERRIDE (e.g. 'explicit', 'intentional', 'reviewed', 'deliberately'), NEVER " +
		"generic scope words like 'all'/'everything'/'every' that appear whenever the constraint itself applies — " +
		"and NO avoid token may appear in any of your positives."
	prompt := fmt.Sprintf("Constraint signature: %q\nReal rule lines that re-encoded it across projects (this is the EVIDENCE the entry is minted from — justification, not serve-time contexts):\n%s\n\nAuthor the entry JSON.",
		signature, strings.Join(evidence, "\n"))

	r, err := client.Complete(context.Background(), c.Model, sys, prompt, 1000)
	if err != nil {
		return fmt.Errorf("authoring entry: %w", err)
	}
	ae, err := parseAuthoredEntry(r.Text)
	if err != nil {
		return fmt.Errorf("model did not return a usable entry: %w", err)
	}
	if ae.MinConf == 0 {
		ae.MinConf = 0.5
	}

	// Coverage is gated against the model's REALISTIC positives — literal contexts
	// a user/assistant would type. The doc rule-lines are discovery evidence
	// (justification that the constraint recurs), NOT serve-time contexts: they are
	// prose that often embeds the remediation language an Avoid token guards on, so
	// folding them in as positives would be self-contradictory. They stay in the
	// authoring prompt only.
	gate := library.GateEntry(ae.Entry, ae.Positives, ae.TooMuchExamples, nil)

	fmt.Printf("=== authored entry %q (rung %s, min_conf %.2f) ===\n", ae.Name, ae.Rung, ae.MinConf)
	fmt.Printf("  match: %s\n  avoid: %s\n  artifact: %s\n\n", strings.Join(ae.Match, " "), strings.Join(ae.Avoid, " "), truncate(ae.Artifact, 100))
	fmt.Printf("=== producer-verifier gate (deterministic, no serve) ===\n")
	for _, line := range gate.Report {
		fmt.Printf("  %s\n", line)
	}
	if !gate.Passed {
		fmt.Printf("\n  → REJECT: the authored entry failed its triggering gate; NOT emitted.\n")
		return nil
	}

	outDir := filepath.Clean(filepath.Join(c.CacheDir, "..", ".crystal-proposals"))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return usageError{err}
	}
	path := filepath.Join(outDir, "library-"+ae.Name+".json")
	doc := struct {
		Entries []library.Entry `json:"entries"`
	}{Entries: []library.Entry{ae.Entry}}
	b, _ := json.MarshalIndent(doc, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return usageError{err}
	}
	fmt.Printf("\n  → PROMOTE (proposal only): gate passed; wrote serve entry to %s\n", path)
	fmt.Printf("  Add it after review, then `crystal library --lib <merged>`. (Cached: %v)\n", r.Cached)
	return nil
}

// parseAuthoredEntry extracts the entry JSON from a model reply (tolerating a
// code fence or surrounding prose).
func parseAuthoredEntry(text string) (authoredEntry, error) {
	s := stripFences(strings.TrimSpace(text))
	if i := strings.IndexByte(s, '{'); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndexByte(s, '}'); j >= 0 {
		s = s[:j+1]
	}
	var ae authoredEntry
	if err := json.Unmarshal([]byte(s), &ae); err != nil {
		return authoredEntry{}, err
	}
	if ae.Name == "" {
		return authoredEntry{}, fmt.Errorf("empty name")
	}
	if len(ae.Match) == 0 {
		return authoredEntry{}, fmt.Errorf("empty match tokens")
	}
	if ae.Artifact == "" {
		return authoredEntry{}, fmt.Errorf("empty artifact body")
	}
	return ae, nil
}
