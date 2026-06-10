package cmd

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/justinstimatze/crystal/internal/llm"
)

// AmortizeCmd answers the question `serve` left open: a promoted artifact isn't
// free to MAKE — the expensive tier paid a one-time authoring round-trip. So
// how many served hits repay that cost, and how often can drift force a
// re-author before the win is erased? This is the breakeven the value prop
// asserts but never showed.
//
// The accounting is pure wall-clock (the thesis axis — token cost is
// collapsing, so it's reported second):
//
//	baseline (no crystal):  N_covered × model_p50          (call the model every covered hit)
//	crystal:                T_author + N_covered × det     (author once, then serve ~free)
//	crystal wins once:      N_covered > T_author / (model_p50 − det)   ≡ the breakeven
//
// And the symmetric drift bound: re-authoring more often than once per
// `breakeven` covered-hits erases the win (same number, the other direction).
//
// All inputs are already in the disk cache (the author call's real LatencyMS,
// the per-command model latencies), so this reconstructs them for free.
type AmortizeCmd struct {
	Corpus   string   `help:"Corpus dir of real records." default:"testdata/corpus"`
	Home     []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir string   `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Sample   int      `help:"Author sample size (must match the author run you want to price)." default:"800"`
	Model    string   `help:"Authoring model (the expensive tier)." default:"claude-opus-4-8"`
	Reps     int      `help:"Iterations to time the deterministic tier over." default:"1000"`
	Probe    int      `help:"Commands to sample for the model-latency baseline (capped — does NOT call the model over the whole corpus)." default:"60"`
}

func (c *AmortizeCmd) Run() error {
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

	// Reconstruct the SAME authored table the `author` run produced (cache hit →
	// the real persisted authoring latency + tokens, no new spend).
	var covered []labeledCmd
	for _, cmd := range cmds {
		if ref := detClassify(cmd); ref != "" {
			covered = append(covered, labeledCmd{cmd, ref})
		}
	}
	var train []labeledCmd
	for i, l := range covered {
		if i%2 == 0 {
			train = append(train, l)
		}
	}
	authorSet := subsample(train, c.Sample)
	_, ares, err := authorRules(ctx, client, c.Model, triageCategories, authorSet)
	if err != nil {
		return usageError{fmt.Errorf("reconstructing authored table: %w", err)}
	}
	if ares.LatencyMS <= 0 {
		return usageError{fmt.Errorf("authoring latency not recorded (run `crystal author --home ... --sample %d` first to populate the cache with a measured call)", c.Sample)}
	}

	// Coverage is free (deterministic) over the WHOLE corpus.
	nCovered := 0
	for _, cmd := range cmds {
		if detClassify(cmd) != "" {
			nCovered++
		}
	}
	// Per-hit model latency baseline: probe only a capped SUBSAMPLE — we need a
	// p50, not a model call per command. (Looping the model over a 21k-command
	// corpus would be tens of thousands of live calls; the cost discipline
	// forbids it.) Spread the probe across the corpus, prefer covered commands.
	var probeCmds []string
	for _, cmd := range cmds {
		if detClassify(cmd) != "" {
			probeCmds = append(probeCmds, cmd)
		}
	}
	probeCmds = subsampleStr(probeCmds, c.Probe)
	var modelLat []int64
	for _, cmd := range probeCmds {
		_, lat := serveModelClassify(ctx, client, cmd)
		if lat > 0 {
			modelLat = append(modelLat, lat)
		}
	}
	if len(modelLat) == 0 {
		return usageError{fmt.Errorf("no model latencies measured over the %d-command probe", len(probeCmds))}
	}
	modelP50 := median(modelLat)

	// Deterministic per-call latency (sub-µs). Per-call cost is independent of
	// corpus size, so time over a capped sample × Reps — looping the whole 21k
	// corpus 1000× is ~22M calls and pure waste.
	timeCmds := subsampleStr(cmds, 200)
	start := time.Now()
	for r := 0; r < c.Reps; r++ {
		for _, cmd := range timeCmds {
			_ = detClassify(cmd)
		}
	}
	detPerCallMS := float64(time.Since(start).Nanoseconds()) / float64(c.Reps) / float64(len(timeCmds)) / 1e6

	tAuthor := float64(ares.LatencyMS)
	savedPerHit := float64(modelP50) - detPerCallMS // latency removed per covered hit
	if savedPerHit <= 0 {
		return usageError{fmt.Errorf("model latency (%dms) not above deterministic (%.4fms) — no saving to amortize", modelP50, detPerCallMS)}
	}
	breakeven := int(math.Ceil(tAuthor / savedPerHit))

	fmt.Printf("amortize: pricing the authored artifact over its served hits (%s)\n\n", src)

	fmt.Println("=== one-time authoring cost (real, from cache) ===")
	fmt.Printf("  author call: %s wall-clock (%s, %d-example sample), %d in / %d out tokens\n",
		fmtNS(ares.LatencyMS*1e6), c.Model, len(authorSet), ares.InputTokens, ares.OutputTokens)

	fmt.Println("\n=== per-hit latency saved (on a covered command) ===")
	fmt.Printf("  model round-trip removed:  ~%dms (Haiku p50 over a %d-command probe) − %.4fms (det) ≈ %.0fms saved/hit\n", modelP50, len(modelLat), detPerCallMS, savedPerHit)

	fmt.Println("\n=== breakeven ===")
	fmt.Printf("  T_author / saved-per-hit = %.0fms / %.0fms = %d covered hits to repay authoring\n", tAuthor, savedPerHit, breakeven)
	fmt.Printf("  (this corpus has %d covered commands; ", nCovered)
	if nCovered >= breakeven {
		fmt.Printf("already %.1f× past breakeven — authoring is repaid)\n", float64(nCovered)/float64(breakeven))
	} else {
		fmt.Printf("%d short of breakeven on a single pass — repays once the chore recurs)\n", breakeven-nCovered)
	}

	fmt.Println("\n=== the drift bound: how often can re-authoring fire before the win is erased? ===")
	fmt.Printf("  re-authoring more often than once per %d covered hits nets NEGATIVE.\n", breakeven)
	fmt.Println("  net latency vs baseline at a few re-author cadences R (covered hits per re-author):")
	for _, R := range []int{breakeven / 4, breakeven, breakeven * 4, breakeven * 20} {
		if R <= 0 {
			continue
		}
		baseline := float64(R) * float64(modelP50)
		crystal := tAuthor + float64(R)*detPerCallMS
		netFrac := (baseline - crystal) / baseline
		verdict := "WIN"
		if netFrac <= 0 {
			verdict = "LOSS"
		}
		fmt.Printf("    R=%-6d  baseline %8.0fms  crystal %8.0fms  net %+5.0f%%  %s\n", R, baseline, crystal, netFrac*100, verdict)
	}

	// Token economics, reported second (the collapsing axis). One covered hit
	// saves one cheap-model call; authoring spent expensive-tier tokens once.
	authCost := tokenCostUSD(c.Model, ares.InputTokens, ares.OutputTokens)
	perHitHaiku := tokenCostUSD(llm.ModelHaiku, 236, 6) // representative cached call (see SERVE_FINDINGS)
	fmt.Println("\n=== token economics (secondary — the collapsing axis) ===")
	fmt.Printf("  authoring: ~$%.5f (Opus, one call).  per covered hit saved: ~$%.6f (a Haiku call).\n", authCost, perHitHaiku)
	if perHitHaiku > 0 {
		fmt.Printf("  token breakeven: ~%d covered hits.\n", int(math.Ceil(authCost/perHitHaiku)))
	}

	fmt.Println("\nThe authored verifier is cheap to make and repays fast in latency; the binding risk is")
	fmt.Println("re-author churn — drift that forces re-authoring faster than the breakeven cadence erases")
	fmt.Println("the win, which is exactly why the demote-on-drift gate (not just detection) is load-bearing.")
	return nil
}

// subsampleStr returns at most n evenly-spaced elements of s (deterministic).
func subsampleStr(s []string, n int) []string {
	if n <= 0 || len(s) <= n {
		return s
	}
	out := make([]string, 0, n)
	step := float64(len(s)) / float64(n)
	for i := 0; i < n; i++ {
		out = append(out, s[int(float64(i)*step)])
	}
	return out
}

// tokenCostUSD prices a call at the published per-1M rates (claude-api skill).
func tokenCostUSD(model string, in, out int64) float64 {
	var pin, pout float64 // $ per 1M tokens
	switch model {
	case llm.ModelOpus:
		pin, pout = 5, 25
	case llm.ModelSonnet:
		pin, pout = 3, 15
	case llm.ModelHaiku:
		pin, pout = 1, 5
	default:
		pin, pout = 5, 25
	}
	return (float64(in)*pin + float64(out)*pout) / 1e6
}
