package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/local"
	"github.com/justinstimatze/crystal/internal/publicai"
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
//
//	reference (default) — the re-author's labels for the NEW class come from
//	  `containerRef`, a provided oracle (the same fidelity-to-reference caveat
//	  `author` carries). The loop closes the WIRING autonomously, but a human/
//	  oracle supplied the new class's ground truth.
//	local — the labels come from 8B+35B LOCAL agreement (no cloud, no human):
//	  agree→label, disagree→abstain. This CLOSES the A5 gap the panel left open —
//	  `containerRef` survives only as a held-out TRUTH yardstick (reported, never
//	  used to decide the swap). The deterministic gate still decides; agreement
//	  only proposes labels for the uncovered region (validated N=250: coverage
//	  0.74, accuracy-on-agree 0.87). Lead the novelty on the gate+demote, not the
//	  agreement (tri-training/QBC prior art) — see PRIOR_ART.md.
type HookLoopCmd struct {
	Corpus        string   `help:"Corpus dir of real records (the normal-regime command stream + authoring train set)." default:"testdata/corpus"`
	Home          []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir      string   `help:"Disk cache dir for LLM calls (re-authoring is cached by content hash)." default:".crystal-cache"`
	Model         string   `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	Sample        int      `help:"Cap on labeled examples shown to the author." default:"200"`
	Threshold     float64  `help:"Re-gate: the re-authored table must cover ≥ this fraction of the drift class to re-promote." default:"0.9"`
	Normal        int      `help:"How many real commands to stream before the injected drift." default:"12"`
	DriftM        int      `help:"Demote after M uncovered commands within the window." default:"3"`
	DriftW        int      `help:"Sliding window size for the drift trigger." default:"5"`
	Oracle        string   `help:"Drift-label oracle: 'reference' (provided containerRef — the documented A5 gap), 'local' (8B+35B agreement, NO cloud), or 'local-confirm' (agreement + a cloud confirm on JUST the abstained slice — targeted spend)." default:"reference" enum:"reference,local,local-confirm"`
	LocalModel    string   `help:"First (small) model for the agreement oracle — always local (Oracle=local)." default:"qwen3:8b"`
	LocalModel2   string   `help:"Second (big) model for the agreement oracle when --big-provider=local." default:"qwen3.6:35b"`
	BigProvider   string   `help:"Where the agreement oracle's SECOND (big) model runs: 'local' (ollama on the house box, spills past 10GB VRAM) or 'publicai' (cloud-OPEN model, no spill stall)." default:"local" enum:"local,publicai"`
	PublicaiModel string   `help:"PublicAI big model when --big-provider=publicai." default:"swiss-ai/apertus-70b-instruct"`
	ConfirmModel  string   `help:"Cloud model that confirms the abstained slice (Oracle=local-confirm). Haiku is cheapest; Opus is the strongest confirm tier." default:"claude-haiku-4-5" enum:"claude-haiku-4-5,claude-sonnet-4-6,claude-opus-4-8"`
	Verbose       bool     `help:"Print the full hook JSON response for every command."`
}

func (c *HookLoopCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	// When the drift-label oracle uses the local agreement (local OR local-confirm),
	// stand up the local client up front and fail loud if the box is unreachable —
	// an unreachable oracle must NOT silently degrade to universal abstention.
	var lc *local.Client
	var pc *publicai.Client
	if c.Oracle == "local" || c.Oracle == "local-confirm" {
		lc, err = local.New(c.CacheDir)
		if err != nil {
			return usageError{err}
		}
		if err := lc.Reachable(ctx); err != nil {
			return usageError{fmt.Errorf("oracle=%s but the local model host is unreachable: %w", c.Oracle, err)}
		}
		// The agreement pair's BIG model can run in the cloud (PublicAI) instead of
		// the spilling local 35B — fail loud up front if its gateway/key is bad.
		if c.BigProvider == "publicai" {
			pc, err = publicai.New(c.CacheDir)
			if err != nil {
				return usageError{err}
			}
			if err := pc.Reachable(ctx); err != nil {
				return usageError{fmt.Errorf("oracle=%s big-provider=publicai but the gateway is unreachable: %w", c.Oracle, err)}
			}
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
	// oracleLabel resolves one drifted command's label under the chosen oracle (the
	// A5 fork). It returns (label, source) where source ∈ {reference, agree, confirm,
	// abstain}; an empty label means abstention (no usable label). The CASCADE:
	//   reference     — provided containerRef (documented gap: a human supplied truth).
	//   local         — 8B+35B agreement only; disagree → abstain (no cloud, no human).
	//   local-confirm — agreement when the models agree (zero cloud), else escalate
	//                   JUST that command to a cloud confirm (Haiku) — targeted spend
	//                   on the small uncertain slice (FrugalGPT/AutoMix), not the whole
	//                   class. containerRef stays a held-out TRUTH yardstick only.
	confirmModel := c.ConfirmModel
	// Build the agreement pair once: small model is always local; the big model's
	// PLACEMENT is the --big-provider switch (local ollama, or cloud-open PublicAI).
	small := localClassifier(lc, c.LocalModel)
	var big classifier
	if c.BigProvider == "publicai" {
		big = publicaiClassifier(pc, c.PublicaiModel)
	} else {
		big = localClassifier(lc, c.LocalModel2)
	}
	oracleLabel := func(cmd string) (label, source string, err error) {
		switch c.Oracle {
		case "local", "local-confirm":
			l, agreed, e := agreementOf(ctx, small, big, cats2, cmd)
			if e != nil {
				return "", "", e
			}
			if agreed {
				return l, "agree", nil
			}
			if c.Oracle == "local-confirm" {
				cl, e := cloudClassifyCats(ctx, client, confirmModel, cats2, cmd)
				if e != nil {
					return "", "", e
				}
				return cl, "confirm", nil
			}
			return "", "abstain", nil
		default: // reference
			return containerRef(cmd), "reference", nil
		}
	}

	// LABEL the captured drift commands.
	labeledDrift, abstained, confirmed := 0, 0, 0
	for _, cmd := range st.RecentUncovered {
		lab, src, lerr := oracleLabel(cmd)
		if lerr != nil {
			return usageError{fmt.Errorf("oracle (%s) on %q: %w", c.Oracle, cmd, lerr)}
		}
		if lab == "" {
			abstained++
			continue // honest abstention — the loop declines to label what it cannot resolve
		}
		if src == "confirm" {
			confirmed++ // a cloud call was spent on this abstained-by-local command
		}
		train2 = append(train2, labeledCmd{cmd, lab})
		labeledDrift++
	}
	pair := fmt.Sprintf("%s + %s", small.name, big.name)
	noCloud := "no cloud"
	if c.BigProvider == "publicai" {
		noCloud = "cloud-open big model, no spill"
	}
	switch c.Oracle {
	case "local":
		fmt.Printf("  oracle=AGREEMENT (%s, %s): labeled %d, abstained on %d → re-authoring with the class in scope\n", pair, noCloud, labeledDrift, abstained)
	case "local-confirm":
		fmt.Printf("  oracle=AGREEMENT+CONFIRM (%s; confirm=%s): %d labeled (%d by agreement + %d by cloud confirm on the abstained slice), %d still abstained → re-authoring\n",
			pair, confirmModel, labeledDrift, labeledDrift-confirmed, confirmed, abstained)
	default:
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
	//   gate  — what the AUTONOMOUS loop can see: where the oracle resolved a label
	//           (incl. cloud-confirmed ones), does v2 match it? Some are NOT in train2
	//           (a real holdout). Pure-abstain commands are not gateable.
	//   truth — the honest yardstick: v2 vs the held-out containerRef ground truth,
	//           NEVER used to decide the swap (that would smuggle the oracle back).
	gateMatched, gateN, truthMatched, overrode := 0, 0, 0, 0
	for _, cmd := range driftCommands {
		if v2.classify(cmd) == containerRef(cmd) {
			truthMatched++
		}
		lab, _, lerr := oracleLabel(cmd)
		if lerr != nil {
			return usageError{fmt.Errorf("oracle (%s, gate) on %q: %w", c.Oracle, cmd, lerr)}
		}
		if lab == "" {
			continue // abstained — not gateable
		}
		gateN++
		vcls := v2.classify(cmd)
		if vcls == lab {
			gateMatched++
			continue
		}
		// v2 DISAGREES with the assigned (cheap local-agreement) label. Rather than
		// auto-reject — which would penalize v2 for CORRECTING a confidently-wrong
		// agreement label (the 0.85-on-agree residual; the finding that motivated
		// this) — escalate JUST this disputed command to the stronger confirm tier as
		// a tiebreak. If the confirm tier backs v2, the cheap label was wrong → trust
		// the stronger producer and OVERRIDE. If it backs the original label, v2 is
		// genuinely wrong → a real miss. Targeted: confirm fires only on conflicts.
		if c.Oracle == "local-confirm" {
			tb, terr := cloudClassifyCats(ctx, client, confirmModel, cats2, cmd)
			if terr != nil {
				return usageError{fmt.Errorf("confirm tiebreak on %q: %w", cmd, terr)}
			}
			if tb == vcls && tb != lab {
				overrode++
				gateMatched++ // the stronger tier agrees with v2; the cheap label was wrong
			}
		}
	}
	gateAcc := float64(gateMatched) / float64(max(gateN, 1))
	truthAcc := float64(truthMatched) / float64(len(driftCommands))
	if overrode > 0 {
		fmt.Printf("  re-gate (oracle-confident, %d/%d covered): %d/%d = %.2f (gate %.2f) — %d cheap label(s) OVERRIDDEN by the confirm tiebreak (v2 corrected the agreement)\n", gateN, len(driftCommands), gateMatched, gateN, gateAcc, c.Threshold, overrode)
	} else {
		fmt.Printf("  re-gate (oracle-confident, %d/%d covered): %d/%d = %.2f (gate %.2f)\n", gateN, len(driftCommands), gateMatched, gateN, gateAcc, c.Threshold)
	}
	if c.Oracle != "reference" {
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
	}
	switch c.Oracle {
	case "local":
		fmt.Printf("  oracle=local: new-class labels from 8B+35B agreement — NO cloud, NO human; v2 vs held-out truth %d/%d.\n", truthMatched, len(driftCommands))
		fmt.Printf("  (Agreement abstains heavily on a NOVEL class — try --oracle local-confirm to recover the abstained slice.)\n")
	case "local-confirm":
		fmt.Printf("  oracle=local-confirm: labels from local agreement + %d cloud confirm call(s) on JUST the abstained slice;\n", confirmed)
		fmt.Printf("  v2 vs held-out truth %d/%d. Targeted spend: cloud paid only where the two local models disagreed.\n", truthMatched, len(driftCommands))
		fmt.Printf("  The deterministic gate still decided the swap; the oracle only PROPOSED labels. (Lead novelty on the gate.)\n")
	default:
		fmt.Println("  (oracle=reference: labels came from a provided reference — for no-oracle discovery use --oracle local / local-confirm.)")
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
