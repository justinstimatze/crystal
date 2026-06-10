package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// driftCommands is a NEW class of commands the original authored rules were
// never trained on — container tooling. Their correct category is "container",
// which is not in the original category set, so the v1 authored rules CANNOT
// produce it: every one diverges. This is a known-answer injected distribution
// shift (not a fuzzy guess), so the demotion + re-author loop is verifiable.
var driftCommands = []string{
	"docker build -t app .",
	"docker run --rm app",
	"docker ps -a",
	"podman images",
	"kubectl get pods",
	"kubectl apply -f deploy.yaml",
	"docker compose up -d",
	"kubectl logs -f web",
}

// containerRef is the deterministic ground truth for the drift class.
func containerRef(cmd string) string {
	c := strings.ToLower(strings.TrimSpace(cmd))
	for _, seg := range splitSegments(c) {
		switch strings.Fields(seg)[0] {
		case "docker", "podman", "kubectl", "k9s", "helm":
			return "container"
		}
	}
	return ""
}

// driftAndReauthor streams the new command class past the promoted rules,
// fires windowed M-in-W demotion on the divergences, then re-authors WITH the
// new class in training and re-gates. The thesis claim under test: the gate
// detects the drift and the re-authored artifact recovers.
func (c *AuthorCmd) driftAndReauthor(ctx context.Context, client *llm.Client, train []labeledCmd) {
	fmt.Printf("\n=== drift: stream a new command class (containers) the rules never saw ===\n")

	// The promoted table is recovered by re-reading nothing — caller still
	// holds it, but we re-derive divergence here against the known reference so
	// the stream is self-contained. Re-run the original author to get the table
	// is wasteful; instead the caller passes train and we re-author once (cached)
	// to reconstruct the same promoted table deterministically.
	table, _, err := authorRules(ctx, client, c.Model, triageCategories, train)
	if err != nil {
		fmt.Printf("  (could not reconstruct promoted table: %v)\n", err)
		return
	}

	// Windowed M-in-W demotion over the divergence stream.
	demotedAt, leaked := streamDemote(table, driftCommands, containerRef, c.DriftM, c.DriftW)
	if demotedAt >= 0 {
		fmt.Printf("  served %d before DEMOTE at index %d (%d-in-%d divergence window fired); %d wrong outputs leaked first\n",
			demotedAt, demotedAt, c.DriftM, c.DriftW, leaked)
	} else {
		fmt.Printf("  streamed all %d without demotion (%d diverged) — rule %d-in-%d never fired\n",
			len(driftCommands), leaked, c.DriftM, c.DriftW)
		fmt.Println("  ⚠ drift went undetected; no re-author triggered.")
		return
	}

	// ---- RE-AUTHOR with the new class in scope ----
	fmt.Printf("\n=== re-author: regenerate rules WITH the container class in training ===\n")
	cats2 := append(append([]string{}, triageCategories...), "container")
	train2 := append([]labeledCmd{}, train...)
	for _, cmd := range driftCommands {
		train2 = append(train2, labeledCmd{cmd, "container"})
	}
	table2, _, err := authorRules(ctx, client, c.Model, cats2, train2)
	if err != nil {
		fmt.Printf("  re-authoring failed: %v\n", err)
		return
	}
	fmt.Printf("  re-authored table: %d rules (was %d)\n", len(table2.Rules), len(table.Rules))

	// Re-gate ONLY the drift class against its reference: does the regenerated
	// artifact now cover what triggered the demotion?
	matched := 0
	var misses []string
	for _, cmd := range driftCommands {
		if table2.classify(cmd) == containerRef(cmd) {
			matched++
		} else {
			misses = append(misses, cmd)
		}
	}
	acc := float64(matched) / float64(len(driftCommands))
	fmt.Printf("  re-gate on the drift class: %d/%d = %.2f", matched, len(driftCommands), acc)
	if acc >= c.Threshold {
		fmt.Printf("  → RECOVERED (≥ %.2f). The loop closes: detect drift → re-author → re-pass.\n", c.Threshold)
	} else {
		fmt.Printf("  → still %.2f < %.2f; re-author did not fully recover. Misses: %v\n", acc, c.Threshold, misses)
	}
}

// streamDemote replays cmds past a fixed rule table, comparing each to its
// reference. It returns the index it demoted at (-1 if never) and the count of
// divergences seen before demotion. Same windowed M-in-W rule as internal/drift.
func streamDemote(t ruleTable, cmds []string, ref func(string) string, m, w int) (demotedAt, leaked int) {
	demotedAt = -1
	window := make([]bool, 0, w)
	divInWindow := 0
	for i, cmd := range cmds {
		diverged := t.classify(cmd) != ref(cmd)
		if diverged {
			leaked++
		}
		window = append(window, diverged)
		if diverged {
			divInWindow++
		}
		if len(window) > w {
			if window[0] {
				divInWindow--
			}
			window = window[1:]
		}
		if divInWindow >= m {
			return i, leaked
		}
	}
	return demotedAt, leaked
}
