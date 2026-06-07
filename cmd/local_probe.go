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
	Model2   string   `help:"Second local model. When set, run the TWO-MODEL AGREEMENT oracle: agree→trust, disagree→abstain/escalate. Reports abstention COVERAGE, not just accuracy-on-agree."`
	N        int      `help:"Cap the covered sample (0 = all covered; local CPU inference is the bottleneck)." default:"0"`
	Offline  bool     `help:"Serve local results ONLY from the disk cache (no GPU box needed); a cache miss is a loud error. Re-validates a prior measured run."`
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
	if c.Offline {
		// Cache-only replay: skip the live reachability check; a miss errors loud.
		lc.SetOffline(true)
	} else if err := lc.Reachable(ctx); err != nil {
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

	if c.Model2 != "" {
		fmt.Printf("local-probe: %d reference-covered commands (%s); cloud=%s local=%s + %s (agreement) @ %s\n\n",
			len(labeled), src, llm.ModelHaiku, c.Model, c.Model2, local.Host())
	} else {
		fmt.Printf("local-probe: %d reference-covered commands (%s); cloud=%s local=%s @ %s\n\n",
			len(labeled), src, llm.ModelHaiku, c.Model, local.Host())
	}

	var haikuLat, localLat []int64
	haikuOK, localOK := 0, 0
	// rows captures each command's labels so the agreement oracle can be computed
	// honestly after the loop (agreement, abstention coverage, accuracy split).
	var rows []probeRow
	for _, l := range labeled {
		hCat, hLat := serveModelClassify(ctx, cloud, l.cmd)
		lCat, lLat, err := localClassify(ctx, lc, c.Model, l.cmd)
		if err != nil {
			return usageError{fmt.Errorf("local classify (%s): %w", c.Model, err)}
		}
		haikuLat = append(haikuLat, hLat)
		localLat = append(localLat, lLat)
		if hCat == l.ref {
			haikuOK++
		}
		if lCat == l.ref {
			localOK++
		}
		r := probeRow{ref: l.ref, m1: lCat}
		if c.Model2 != "" {
			m2Cat, _, err := localClassify(ctx, lc, c.Model2, l.cmd)
			if err != nil {
				return usageError{fmt.Errorf("local classify (%s): %w", c.Model2, err)}
			}
			r.m2 = m2Cat
		}
		rows = append(rows, r)
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

	if c.Model2 != "" {
		reportAgreement(c.Model, c.Model2, rows)
	}

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
		// Don't assert the remote host's hardware — the client only knows OLLAMA_HOST.
		// Slow local p50 is usually a model-too-big-for-VRAM offload, not GPU absence;
		// right-size the model to the card before concluding the tier is slow.
		fmt.Printf("But local p50 %dms > cloud %dms (host %s) — if the model exceeds VRAM it offloads to CPU; try a model that fits resident.\n", lp50, hp50, local.Host())
	}
	// Oracle viability: the same accuracy number answers whether the local model
	// could LABEL a new class for the hook-loop re-author without a cloud call.
	fmt.Printf("  live-oracle (for hook-loop re-author labels): ")
	switch {
	case localAcc >= 0.90:
		fmt.Printf("PLAUSIBLE — %.2f vs the deterministic reference; good enough to propose labels behind the gate.\n", localAcc)
	case localAcc >= haikuAcc-0.05:
		fmt.Printf("BELOW the 0.90 bar at %.2f, but it TIES cloud-cheap (Haiku %.2f) — so the gap is the\n", localAcc, haikuAcc)
		fmt.Printf("    reference's own debatable edge conventions, not a weak model. A confirm step (local proposes,\n")
		fmt.Printf("    a cheap cloud call ratifies) is the viable oracle path; raw 0.90-vs-det is the wrong bar.\n")
	default:
		fmt.Printf("TOO WEAK at %.2f — a label source this noisy would feed the gate bad training; the new-class\n", localAcc)
		fmt.Printf("    oracle gap stays open (needs a stronger model, a model that fits VRAM, +LoRA, or a confirm step).\n")
	}
	return nil
}

// reportAgreement prints the two-model agreement oracle: the all-local trust
// signal where two independent local models AGREEING on a label is the signal to
// trust it (no cloud), and DISAGREEING is the signal to abstain/escalate. This is
// tri-training (Zhou & Li 2005) at N=2 / Query-by-Committee — so the honest report
// is abstention COVERAGE (the fraction we can label at all), not just the inflated
// accuracy on the agreed subset. It feeds the hook-loop re-author as a label source
// behind crystal's deterministic gate (the gate, not the agreement, is the novelty).
// probeRow is one command's reference label plus each local model's category,
// captured during the probe loop so the agreement oracle is computed from data.
type probeRow struct{ ref, m1, m2 string }

func reportAgreement(m1, m2 string, rows []probeRow) {
	n := len(rows)
	agree, agreeOK := 0, 0   // agree: both models emitted the SAME label; agreeOK: that label == det reference
	disagree, disagreeM1OK, disagreeM2OK := 0, 0, 0
	m1OK, m2OK := 0, 0
	for _, r := range rows {
		if r.m1 == r.ref {
			m1OK++
		}
		if r.m2 == r.ref {
			m2OK++
		}
		if r.m1 != "" && r.m1 == r.m2 {
			agree++
			if r.m1 == r.ref {
				agreeOK++
			}
		} else {
			disagree++
			if r.m1 == r.ref {
				disagreeM1OK++
			}
			if r.m2 == r.ref {
				disagreeM2OK++
			}
		}
	}
	fmt.Printf("\n=== two-model agreement (all-local label oracle) ===\n")
	fmt.Printf("  models: %s  vs  %s   (agree→trust · disagree→abstain/escalate)\n", m1, m2)
	fmt.Printf("  N covered                 : %d\n", n)
	fmt.Printf("  solo accuracy vs det      : %s %.2f · %s %.2f\n", m1, ratio(m1OK, n), m2, ratio(m2OK, n))
	fmt.Printf("  COVERAGE (agree rate)     : %d/%d = %.2f   ← trust this fraction; abstain/escalate the rest\n", agree, n, ratio(agree, n))
	fmt.Printf("  accuracy ON AGREE         : %d/%d = %.2f   ← the label quality on the covered fraction\n", agreeOK, agree, ratio(agreeOK, agree))
	fmt.Printf("  accuracy ON DISAGREE      : %s %d/%d · %s %d/%d   ← the escalated (abstained) set\n",
		m1, disagreeM1OK, disagree, m2, disagreeM2OK, disagree)
	// The oracle is useful iff agreement CONCENTRATES correctness: accuracy-on-agree
	// must beat both solo accuracies, and the abstained set is where the errors live.
	concentrates := ratio(agreeOK, agree) > ratio(m1OK, n) && ratio(agreeOK, agree) > ratio(m2OK, n)
	fmt.Printf("  oracle read: ")
	switch {
	case agree == 0:
		fmt.Printf("models NEVER agree — no covered fraction; not a usable oracle on this corpus.\n")
	case concentrates:
		fmt.Printf("agreement CONCENTRATES correctness — %.2f on agree > both solo (%.2f, %.2f).\n", ratio(agreeOK, agree), ratio(m1OK, n), ratio(m2OK, n))
		fmt.Printf("    Use as hook-loop label oracle on the %.0f%% it covers (zero cloud); route the abstained %.0f%% to a stronger tier.\n", ratio(agree, n)*100, ratio(disagree, n)*100)
	default:
		fmt.Printf("agreement does NOT concentrate correctness here (%.2f on agree vs solo %.2f/%.2f) — coverage may be too high to be selective.\n", ratio(agreeOK, agree), ratio(m1OK, n), ratio(m2OK, n))
	}
}

// ratio is a divide-by-zero-safe rate for the agreement report.
func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// localClassify runs the local cheap-model baseline for one command and returns
// its parsed category plus the REAL measured latency (cached after first call).
func localClassify(ctx context.Context, lc *local.Client, model, cmd string) (string, int64, error) {
	sys := "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		strings.Join(triageCategories, ", ") + "."
	r, err := lc.Classify(ctx, model, sys, cmd, 16)
	if err != nil {
		return "", 0, err
	}
	return parseCategory(r.Text), r.LatencyMS, nil
}
