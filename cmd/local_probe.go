package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/local"
)

// LocalProbeCmd is the A5 probe (the sovereignty rung, de-risked before any full
// slice — the way `probe` de-risked the cloud tier). The cost gradient is
// frontier → cloud-cheap (Haiku) → LOCAL (a small ollama model) → deterministic.
// Every rung left of the deterministic tier currently uses cloud Haiku; this
// probe asks the one question A5 turns on: can a LOCAL small model do the
// categorize chore well enough, and fast enough, to be the cheap tier — and, by
// the same number, to be the live ORACLE the `hook-loop` re-author still lacks?
//
// It measures all three tiers on the COVERED fraction, because that is the only
// place with trustworthy ground truth: there `detClassify` IS the reference. So
// the gradeable question is "does the local model match the cloud cheap model's
// accuracy vs the deterministic reference, at acceptable local latency?" A high
// match → local is a plausible cheap-tier AND oracle. A low match → a 1.5B-class
// local model is too weak for this chore (and the residual, which is HARDER than
// the covered fraction, would be worse still).
type LocalProbeCmd struct {
	Corpus   string   `help:"Corpus dir of real records." default:"testdata/corpus"`
	Home     []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir string   `help:"Disk cache dir (shared with the cloud cache; local keys are namespaced)." default:".crystal-cache"`
	Model    string   `help:"Local ollama model (the cheap tier under test)." default:"qwen2:1.5b"`
	N        int      `help:"Cap the covered sample (0 = all covered; local CPU inference is the bottleneck)." default:"0"`
	Verbose  bool     `help:"Per-command reference / Haiku / local categories and the local latency."`
}

func (c *LocalProbeCmd) Run() error {
	cloud, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	lc, err := local.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	if err := lc.Reachable(ctx); err != nil {
		return usageError{err}
	}

	cmds, src, err := loadBashCommands(c.Corpus, c.Home)
	if err != nil {
		return usageError{err}
	}
	// The gradeable set: commands the deterministic reference covers (ground truth).
	var labeled []labeledCmd
	for _, cmd := range cmds {
		if ref := detClassify(cmd); ref != "" {
			labeled = append(labeled, labeledCmd{cmd, ref})
		}
	}
	if len(labeled) == 0 {
		return usageError{fmt.Errorf("no reference-covered commands in %s", src)}
	}
	labeled = subsample(labeled, c.N)

	fmt.Printf("local-probe: %d reference-covered commands (%s); cloud=%s local=%s @ %s\n\n",
		len(labeled), src, llm.ModelHaiku, c.Model, local.Host())

	var haikuLat, localLat []int64
	haikuOK, localOK := 0, 0
	for _, l := range labeled {
		hCat, hLat := serveModelClassify(ctx, cloud, l.cmd)
		lCat, lLat := localClassify(ctx, lc, c.Model, l.cmd)
		haikuLat = append(haikuLat, hLat)
		localLat = append(localLat, lLat)
		if hCat == l.ref {
			haikuOK++
		}
		if lCat == l.ref {
			localOK++
		}
		if c.Verbose {
			hMark, lMark := "✓", "✓"
			if hCat != l.ref {
				hMark = "✗"
			}
			if lCat != l.ref {
				lMark = "✗"
			}
			fmt.Printf("  ref=%-13s  haiku %s %-13s  local %s %-13s %5dms  %s\n",
				l.ref, hMark, hCat, lMark, lCat, lLat, truncate(l.cmd, 40))
		}
	}

	n := len(labeled)
	haikuAcc := float64(haikuOK) / float64(n)
	localAcc := float64(localOK) / float64(n)

	fmt.Printf("\n=== accuracy vs the deterministic reference (on the covered fraction) ===\n")
	fmt.Printf("  deterministic (detClassify):  %d/%d = 1.00   (IS the reference; ~µs, 0 spend)\n", n, n)
	fmt.Printf("  cloud cheap (Haiku):          %d/%d = %.2f\n", haikuOK, n, haikuAcc)
	fmt.Printf("  LOCAL (%s):            %d/%d = %.2f\n", c.Model, localOK, n, localAcc)

	fmt.Printf("\n=== latency (real, measured; cached after first call) ===\n")
	fmt.Printf("  cloud cheap (Haiku):  p50 %dms  p99 %dms\n", percentile(haikuLat, 0.50), percentile(haikuLat, 0.99))
	fmt.Printf("  LOCAL (%s):   p50 %dms  p99 %dms\n", c.Model, percentile(localLat, 0.50), percentile(localLat, 0.99))

	fmt.Printf("\n=== verdict ===\n")
	// Cheap-tier viability: matches (or beats) the cloud cheap model's accuracy.
	switch {
	case localAcc >= haikuAcc:
		fmt.Printf("  cheap-tier: VIABLE on accuracy — local %.2f ≥ Haiku %.2f. ", localAcc, haikuAcc)
	case localAcc >= haikuAcc-0.05:
		fmt.Printf("  cheap-tier: BORDERLINE — local %.2f vs Haiku %.2f (within 5pp). ", localAcc, haikuAcc)
	default:
		fmt.Printf("  cheap-tier: NOT viable on accuracy — local %.2f << Haiku %.2f. ", localAcc, haikuAcc)
	}
	if lp50, hp50 := percentile(localLat, 0.50), percentile(haikuLat, 0.50); lp50 <= hp50 {
		fmt.Printf("Local p50 %dms ≤ cloud %dms (latency favors local).\n", lp50, hp50)
	} else {
		fmt.Printf("But local p50 %dms > cloud %dms — on THIS host (CPU, no GPU) local is slower.\n", lp50, hp50)
	}
	// Oracle viability: the same accuracy number answers whether the local model
	// could LABEL a new class for the hook-loop re-author without a cloud call.
	fmt.Printf("  live-oracle (for hook-loop re-author labels): ")
	switch {
	case localAcc >= 0.90:
		fmt.Printf("PLAUSIBLE — %.2f vs the deterministic reference; good enough to propose labels behind the gate.\n", localAcc)
	default:
		fmt.Printf("TOO WEAK at %.2f — a label source this noisy would feed the gate bad training; the new-class\n", localAcc)
		fmt.Printf("    oracle gap stays open (needs a stronger local model, GPU, +LoRA, or a confirm step).\n")
	}
	return nil
}

// localClassify runs the local cheap-model baseline for one command and returns
// its parsed category plus the REAL measured latency (cached after first call).
func localClassify(ctx context.Context, lc *local.Client, model, cmd string) (string, int64) {
	sys := "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		strings.Join(triageCategories, ", ") + "."
	r, err := lc.Classify(ctx, model, sys, cmd, 16)
	if err != nil {
		return "", 0
	}
	return parseCategory(r.Text), r.LatencyMS
}
