package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/local"
)

// HookLoopCmd is the seam the 2026-05-29 panel exposed, wired shut: the live
// PreToolUse hook's demote-on-drift connected to `author`'s re-author, so the
// loop closes AUTONOMOUSLY across real process boundaries — no human in the
// middle, and no terminal demotion.
//
// What the old `hook-demo` showed was DETECTION only: it drove the real binary,
// the tier demoted on a container burst, and then died forever (the panel's
// terminal-DoS; "the loop closes live" was retracted). `hook-loop` adds the
// missing half:
//
//  1. AUTHOR  — Opus authors a v1 rule table from the corpus; it is written to a
//     rule-table ARTIFACT the hook serves from (not the compiled baseline).
//  2. SERVE   — drive the real `crystal hook --rules <artifact>` over normal
//     commands (served from the artifact, 0 model calls) ...
//  3. DEMOTE  — ... then a container burst the v1 rules never saw → the hook
//     DEMOTES live (cross-process, via the on-disk state) and captures
//     the drifted commands into state.recent_uncovered.
//  4. REAUTHOR— the orchestrator reads the demotion + the captured drift sample,
//     re-authors WITH the new class in scope (Opus, cached), and GATES
//     the result on the drift class.
//  5. SWAP    — on a passing gate it atomically swaps the artifact and RE-PROMOTES
//     the tier (clears Demoted — the inverse the old hook lacked).
//  6. RESUME  — re-drive the SAME container commands: now SERVED as "container".
//     The loop closed, live, across separate processes.
//
// The drift-label oracle (the A5 fork, --oracle):
//   reference (default) — the re-author's labels for the NEW class come from
//     `containerRef`, a provided oracle (the same fidelity-to-reference caveat
//     `author` carries). The loop closes the WIRING autonomously, but a human/
//     oracle supplied the new class's ground truth.
//   local — the labels come from 8B+35B LOCAL agreement (no cloud, no human):
//     agree→label, disagree→abstain. This CLOSES the A5 gap the panel left open —
//     `containerRef` survives only as a held-out TRUTH yardstick (reported, never
//     used to decide the swap). The deterministic gate still decides; agreement
//     only proposes labels for the uncovered region (validated N=250: coverage
//     0.74, accuracy-on-agree 0.87). Lead the novelty on the gate+demote, not the
//     agreement (tri-training/QBC prior art) — see PRIOR_ART.md.
type HookLoopCmd struct {
	Corpus    string   `help:"Corpus dir of real records (the normal-regime command stream + authoring train set)." default:"testdata/corpus"`
	Home      []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir  string   `help:"Disk cache dir for LLM calls (re-authoring is cached by content hash)." default:".crystal-cache"`
	Model     string   `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	Sample    int      `help:"Cap on labeled examples shown to the author." default:"200"`
	Threshold float64  `help:"Re-gate: the re-authored table must cover ≥ this fraction of the drift class to re-promote." default:"0.9"`
	Normal    int      `help:"How many real commands to stream before the injected drift." default:"12"`
	DriftM    int      `help:"Demote after M uncovered commands within the window." default:"3"`
	DriftW    int      `help:"Sliding window size for the drift trigger." default:"5"`
	Oracle    string   `help:"Drift-label oracle: 'reference' (the provided containerRef — the documented A5 gap) or 'local' (8B+35B agreement — NO cloud, NO human; the gap closed)." default:"reference" enum:"reference,local"`
	LocalModel  string `help:"First local model for the agreement oracle (Oracle=local)." default:"qwen3:8b"`
	LocalModel2 string `help:"Second local model for the agreement oracle (Oracle=local)." default:"qwen3.6:35b"`
	Verbose   bool     `help:"Print the full hook JSON response for every command."`
}

func (c *HookLoopCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	// When the drift-label oracle is the all-local agreement, stand up the local
	// client up front and fail loud if the box is unreachable — an unreachable
	// oracle must NOT silently degrade to universal abstention.
	var lc *local.Client
	if c.Oracle == "local" {
		lc, err = local.New(c.CacheDir)
		if err != nil {
			return usageError{err}
		}
		if err := lc.Reachable(ctx); err != nil {
			return usageError{fmt.Errorf("oracle=local but the local model host is unreachable: %w", err)}
		}
	}
	cmds, src, err := loadBashCommands(c.Corpus, c.Home)
	if err != nil {
		return usageError{err}
	}
	if len(cmds) == 0 {
		return usageError{fmt.Errorf("no Bash commands found in %s", src)}
	}

	// Authoring train set: reference-covered commands (detClassify is the oracle
	// for the known classes), deterministic even-index split, capped sample —
	// identical construction to `author` so re-author calls hit the same cache.
	var covered []labeledCmd
	var coveredCmds []string
	for _, cmd := range cmds {
		if ref := detClassify(cmd); ref != "" {
			covered = append(covered, labeledCmd{cmd, ref})
			coveredCmds = append(coveredCmds, cmd)
		}
	}
	if len(covered) < 8 {
		return usageError{fmt.Errorf("only %d reference-covered commands; too few to author", len(covered))}
	}
	var train []labeledCmd
	for i, l := range covered {
		if i%2 == 0 {
			train = append(train, l)
		}
	}
	authorSet := subsample(train, c.Sample)

	// Temp artifact + state files: the cross-process substrate.
	dir, err := os.MkdirTemp("", "crystal-hookloop-*")
	if err != nil {
		return usageError{fmt.Errorf("temp dir: %w", err)}
	}
	defer os.RemoveAll(dir)
	artifactPath := dir + "/rules.json"
	statePath := dir + "/state.json"
	self := os.Args[0]

	fmt.Printf("hook-loop: closing the detect→re-author→re-promote loop across REAL process boundaries\n")
	fmt.Printf("artifact (swapped live): %s\nstate (cross-process drift window): %s\n\n", artifactPath, statePath)

	// ---- 1. AUTHOR v1 → write the artifact the hook serves from ----
	fmt.Printf("=== 1. author v1 rule table from %d train commands (%s) ===\n", len(authorSet), src)
	v1, _, err := authorRules(ctx, client, c.Model, triageCategories, authorSet)
	if err != nil {
		return usageError{fmt.Errorf("authoring v1: %w", err)}
	}
	if err := writeRuleArtifact(artifactPath, v1); err != nil {
		return usageError{err}
	}
	fmt.Printf("  authored %d rules → wrote artifact. The hook now serves from THIS, not the compiled baseline.\n\n", len(v1.Rules))

	// ---- 2+3. SERVE then DEMOTE: drive the real binary over normal + drift ----
	normal := pickCovered(coveredCmds, v1, c.Normal)
	stream := append(append([]string{}, normal...), driftCommands...)
	driftStart := len(normal)
	fmt.Printf("=== 2/3. drive `crystal hook --rules <artifact>` over %d live events (serve, then container drift) ===\n", len(stream))
	demotedAt := -1
	for i, cmd := range stream {
		if i == driftStart {
			fmt.Printf("  --- injected drift: %d container commands the v1 rules never saw ---\n", len(driftCommands))
		}
		ctxText, raw, err := invokeHook(self, statePath, artifactPath, cmd, c.DriftM, c.DriftW)
		if err != nil {
			return usageError{fmt.Errorf("invoking hook on %q: %w", cmd, err)}
		}
		label := labelFor(ctxText)
		if strings.Contains(ctxText, "DEMOTED") && demotedAt < 0 {
			demotedAt = i
		}
		fmt.Printf("  [%2d] %-12s %s\n", i, label, truncate(cmd, 46))
		if c.Verbose {
			fmt.Printf("        → %s\n", strings.TrimSpace(raw))
		}
	}
	if demotedAt < 0 {
		fmt.Println("\n  (no demotion — the burst didn't collapse coverage; widen the drift class or tighten M/W.)")
		return nil
	}

	// ---- 4. REAUTHOR: read the demotion + captured drift sample, re-author ----
	st, err := loadHookState(statePath)
	if err != nil {
		return usageError{err}
	}
	fmt.Printf("\n=== 4. re-author: the hook DEMOTED (%s gate) and captured %d drifted commands; re-authoring ===\n",
		st.DemoteReason, len(st.RecentUncovered))
	cats2 := append(append([]string{}, triageCategories...), "container")
	train2 := append([]labeledCmd{}, authorSet...)
	// LABEL the captured drift commands. Two oracle modes (the A5 fork):
	//   reference — the provided containerRef (the documented gap: a human/oracle
	//               supplied the new class's ground truth).
	//   local     — 8B+35B agreement (NO cloud, NO human): agree→label, disagree→
	//               abstain. This is the gap closed; containerRef survives ONLY as
	//               the held-out TRUTH yardstick below, never as the label source.
	labeledDrift, abstained := 0, 0
	for _, cmd := range st.RecentUncovered {
		var lab string
		if c.Oracle == "local" {
			l, agreed, lerr := agreementLabel(ctx, lc, c.LocalModel, c.LocalModel2, cats2, cmd)
			if lerr != nil {
				return usageError{fmt.Errorf("local agreement oracle on %q: %w", cmd, lerr)}
			}
			if !agreed {
				abstained++
				continue // honest abstention — the loop declines to label what its models dispute
			}
			lab = l
		} else {
			lab = containerRef(cmd) // provided oracle for the new class (documented gap)
			if lab == "" {
				continue
			}
		}
		train2 = append(train2, labeledCmd{cmd, lab})
		labeledDrift++
	}
	if c.Oracle == "local" {
		fmt.Printf("  oracle=LOCAL (8B+35B agreement, no cloud): labeled %d captured drift commands, abstained on %d → re-authoring with the class in scope\n", labeledDrift, abstained)
	} else {
		fmt.Printf("  oracle=reference: labeled %d of the captured drift commands as 'container' → re-authoring with the class in scope\n", labeledDrift)
	}
	if labeledDrift == 0 {
		fmt.Printf("  → REJECT: the oracle labeled NO drift commands (all abstained / uncovered); tier stays demoted (no bad swap).\n")
		return nil
	}
	v2, _, err := authorRules(ctx, client, c.Model, cats2, train2)
	if err != nil {
		return usageError{fmt.Errorf("re-authoring: %w", err)}
	}

	// DUAL-SCORE the re-authored table over the FULL drift class:
	//   gate  — what the AUTONOMOUS loop can see: where the oracle is CONFIDENT
	//           (reference always; local only where the two models agree — some of
	//           which are NOT in train2, a real holdout), does v2 match it?
	//   truth — the honest yardstick: v2 vs the held-out containerRef ground truth,
	//           NEVER used to decide the swap (that would smuggle the oracle back).
	gateMatched, gateN, truthMatched := 0, 0, 0
	for _, cmd := range driftCommands {
		if v2.classify(cmd) == containerRef(cmd) {
			truthMatched++
		}
		if c.Oracle == "local" {
			l, agreed, lerr := agreementLabel(ctx, lc, c.LocalModel, c.LocalModel2, cats2, cmd)
			if lerr != nil {
				return usageError{fmt.Errorf("local agreement oracle (gate) on %q: %w", cmd, lerr)}
			}
			if !agreed {
				continue
			}
			gateN++
			if v2.classify(cmd) == l {
				gateMatched++
			}
		} else {
			gateN++
			if v2.classify(cmd) == containerRef(cmd) {
				gateMatched++
			}
		}
	}
	gateAcc := float64(gateMatched) / float64(max(gateN, 1))
	truthAcc := float64(truthMatched) / float64(len(driftCommands))
	fmt.Printf("  re-gate (oracle-confident, %d/%d covered): %d/%d = %.2f (gate %.2f)\n", gateN, len(driftCommands), gateMatched, gateN, gateAcc, c.Threshold)
	if c.Oracle == "local" {
		fmt.Printf("  held-out TRUTH yardstick (NOT used to decide): v2 vs containerRef = %d/%d = %.2f\n", truthMatched, len(driftCommands), truthAcc)
	}
	if gateN == 0 || gateAcc < c.Threshold {
		fmt.Printf("  → REJECT: re-authored table does not cover the oracle-confident drift class; tier stays demoted (no bad swap).\n")
		return nil
	}

	// ---- 5. SWAP + RE-PROMOTE ----
	demoteReason := st.DemoteReason // repromote clears it; keep it for the outcome line
	if err := writeRuleArtifact(artifactPath, v2); err != nil {
		return usageError{err}
	}
	repromote(st)
	if err := saveHookState(statePath, st); err != nil {
		return usageError{err}
	}
	fmt.Printf("  → PROMOTE: atomically swapped the artifact (%d rules) and RE-PROMOTED the tier (Demoted cleared).\n\n", len(v2.Rules))

	// ---- 6. RESUME: re-drive the SAME container commands; now they SERVE ----
	fmt.Printf("=== 6. resume: re-drive the SAME container commands through the live hook ===\n")
	servedNow := 0
	for i, cmd := range driftCommands {
		ctxText, raw, err := invokeHook(self, statePath, artifactPath, cmd, c.DriftM, c.DriftW)
		if err != nil {
			return usageError{fmt.Errorf("invoking hook on %q: %w", cmd, err)}
		}
		label := labelFor(ctxText)
		if strings.Contains(ctxText, "category") {
			servedNow++
		}
		fmt.Printf("  [%2d] %-12s %s\n", i, label, truncate(cmd, 46))
		if c.Verbose {
			fmt.Printf("        → %s\n", strings.TrimSpace(raw))
		}
	}

	fmt.Printf("\n=== outcome ===\n")
	fmt.Printf("  demoted live at stream index %d, re-authored (%s gate), re-promoted, and now serve %d/%d of the\n",
		demotedAt, demoteReason, servedNow, len(driftCommands))
	fmt.Printf("  once-drifting container commands deterministically (0 model calls) — the loop CLOSED across\n")
	fmt.Printf("  %d separate hook processes, autonomously, with no human re-running `author`.\n", len(stream)+len(driftCommands))
	if servedNow == len(driftCommands) {
		fmt.Println("  Terminal demotion is fixed: the tier recovered itself.")
		if c.Oracle == "local" {
			fmt.Printf("  ⇒ A5 GAP CLOSED: the new class's labels came from LOCAL 8B+35B agreement — no cloud, no human —\n")
			fmt.Printf("     and v2 matched the held-out ground truth %d/%d. The deterministic gate decided the swap;\n", truthMatched, len(driftCommands))
			fmt.Printf("     agreement only PROPOSED labels. (Validated oracle: N=250, coverage 0.74 / on-agree 0.87.)\n")
		} else {
			fmt.Println("  (Caveat: the new class's labels came from a provided reference — discovering ground")
			fmt.Println("   truth with no oracle is the local-agreement path: re-run with --oracle local.)")
		}
	}
	return nil
}

// pickCovered returns up to n commands the v1 table actually covers, so the
// normal-regime stream is genuinely served (not accidental residual).
func pickCovered(cmds []string, t ruleTable, n int) []string {
	var out []string
	for _, c := range cmds {
		if t.classify(c) != "" {
			out = append(out, c)
			if len(out) >= n {
				break
			}
		}
	}
	if len(out) == 0 { // fall back to whatever we have
		return subsampleStr(cmds, n)
	}
	return out
}

func labelFor(ctxText string) string {
	switch {
	case strings.Contains(ctxText, "DEMOTED"):
		return "DEMOTE!     "
	case strings.Contains(ctxText, "category"):
		return "serve-det   "
	default:
		return "defer→model "
	}
}
