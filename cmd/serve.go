package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/justinstimatze/crystal/internal/llm"
)

// ServeCmd measures the value prop the whole project asserts but had never
// shown: when a promoted deterministic tier ANSWERS in place of a model call,
// what does shift-left actually buy? Everything before this was a batch over a
// corpus; this serves the deterministic verifier (`detClassify`, the shipped
// hand rules `triage` ships and `author` reproduced) on the covered fraction
// and measures latency before vs after, determinism, and the amortization
// point, against the cheap-model baseline.
//
// The chore is the same one `triage` ships: categorize Bash commands. The
// baseline is "call the cheap model (Haiku) for every command." The served
// pipeline is "deterministic rule answers the covered fraction (g), the model
// only sees the residual." The shift-left win is the model latency removed on
// the covered fraction — and it is LOSSLESS there, because the deterministic
// rule IS the reference answer (no quality trade on what it covers).
type ServeCmd struct {
	Corpus   string   `help:"Corpus dir of real records." default:"testdata/corpus"`
	Home     []string `help:"Instead of the corpus, scan these home dirs' live transcripts. Repeatable."`
	CacheDir string   `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Reps     int      `help:"Iterations to time the deterministic tier over (it is sub-microsecond; repeat to measure honestly)." default:"1000"`
	Verbose  bool     `help:"Per-command served category, model category, and the model's measured latency."`
}

func (c *ServeCmd) Run() error {
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

	// Baseline: the cheap model classifies every command. Latencies are REAL
	// (persisted in the cache from the original live call — see llm.Result),
	// so a cached rerun still reports the measured wall-clock.
	var modelLat []int64 // per-command model latency (ms)
	covered := 0
	type row struct {
		cmd, det, model string
		latMS           int64
	}
	var rows []row
	for _, cmd := range cmds {
		det := detClassify(cmd)
		if det != "" {
			covered++
		}
		cat, lat := serveModelClassify(ctx, client, cmd)
		modelLat = append(modelLat, lat)
		rows = append(rows, row{cmd, det, cat, lat})
	}
	n := len(cmds)
	residual := n - covered

	// Deterministic tier latency: classify is sub-microsecond, so timing a
	// single call is pure noise. Time the whole corpus Reps times and divide.
	start := time.Now()
	for r := 0; r < c.Reps; r++ {
		for _, cmd := range cmds {
			_ = detClassify(cmd)
		}
	}
	detPerCallNS := time.Since(start).Nanoseconds() / int64(c.Reps) / int64(n)

	// Determinism: the served tier must be exactly reproducible. Run twice and
	// assert byte-identical categories (the determinism value-prop, asserted,
	// not assumed). The model tier's determinism is NOT measured here — the
	// cache makes reruns identical, so we cannot honestly claim it from cache.
	det1 := make([]string, n)
	det2 := make([]string, n)
	for i, cmd := range cmds {
		det1[i] = detClassify(cmd)
		det2[i] = detClassify(cmd)
	}
	exactRepro := true
	for i := range det1 {
		if det1[i] != det2[i] {
			exactRepro = false
			break
		}
	}

	modelP50 := median(modelLat)
	modelP99 := percentile(modelLat, 0.99)
	// Blended pipeline latency. Before: every command pays a model call. After:
	// the covered fraction is answered deterministically (≈0), only the residual
	// pays the model. We charge the residual the SAME per-command model cost.
	beforeMS := int64(n) * modelP50
	afterMS := int64(residual) * modelP50
	savedFrac := 0.0
	if beforeMS > 0 {
		savedFrac = float64(beforeMS-afterMS) / float64(beforeMS)
	}

	if c.Verbose {
		fmt.Printf("=== per-command (source: %s) ===\n", src)
		for _, r := range rows {
			d := r.det
			served := "model"
			if d == "" {
				d = "—(residual)"
			} else {
				served = "det"
			}
			fmt.Printf("  served-by=%-5s det=%-13s model=%-13s model_lat=%dms  %s\n",
				served, d, r.model, r.latMS, truncate(r.cmd, 40))
		}
		fmt.Println()
	}

	fmt.Printf("serve: %d real Bash commands (%s) — deterministic tier vs cheap-model baseline\n\n", n, src)

	fmt.Println("=== coverage ===")
	fmt.Printf("  deterministic tier covers %d/%d = g %.2f (model handles the %d-command residual)\n\n", covered, n, frac(covered, n), residual)

	fmt.Println("=== per-call latency (measured) ===")
	fmt.Printf("  deterministic classify:  %s/call  (timed over %d×%d calls)\n", fmtNS(detPerCallNS), c.Reps, n)
	fmt.Printf("  cheap model (Haiku):     p50 %dms, p99 %dms  (real, from cache)\n", modelP50, modelP99)
	if detPerCallNS > 0 {
		fmt.Printf("  speedup on the covered fraction: ~%d× (model p50 %dms vs det %s)\n", (modelP50*1_000_000)/detPerCallNS, modelP50, fmtNS(detPerCallNS))
	}
	fmt.Println()

	fmt.Println("=== blended pipeline latency: serve-det-first vs all-model ===")
	fmt.Printf("  before (model on all %d):        %d ms\n", n, beforeMS)
	fmt.Printf("  after  (det covers %d, model %d): %d ms\n", covered, residual, afterMS)
	fmt.Printf("  latency removed: %.0f%% (≈ g; the model call is deleted on the covered fraction, losslessly)\n\n", savedFrac*100)

	fmt.Println("=== determinism ===")
	if exactRepro {
		fmt.Println("  deterministic tier: exact-repro ✓ (identical over 2 runs) — repeatable, no sampling variance")
	} else {
		fmt.Println("  deterministic tier: NON-reproducible ✗ — this should never happen for a pure rule table")
	}
	fmt.Println("  cheap model: determinism NOT measured (cache makes reruns identical; would need live re-sampling)")
	fmt.Println()

	fmt.Println("Shift-left, served: on the covered fraction the deterministic tier deletes the model")
	fmt.Printf("round-trip entirely (%s vs %dms) at zero quality cost (the rule IS the reference). The\n", fmtNS(detPerCallNS), modelP50)
	fmt.Println("residual is the binding constraint — it still pays the model — so coverage g is the lever.")
	fmt.Println("Caveat: a local microbenchmark, not a live PreToolUse hook in a real agentic loop yet.")
	return nil
}

// serveModelClassify runs the cheap-model baseline for one command and returns
// its category plus the REAL measured latency (cached after the first call).
func serveModelClassify(ctx context.Context, c *llm.Client, cmd string) (string, int64) {
	sys := "Classify this shell command into EXACTLY ONE category, reply with only the category word: " +
		"git, build/test, search/inspect, file-edit, install, nav, network, other."
	r, err := c.Classify(ctx, llm.ModelHaiku, sys, cmd, 8)
	if err != nil {
		return "", 0
	}
	return parseCategory(r.Text), r.LatencyMS
}

// parseCategory maps a model reply to a known category (substring match), or
// returns the lowercased raw text if none matches (schema-invalid → visible).
func parseCategory(text string) string {
	got := strings.ToLower(strings.TrimSpace(text))
	for _, cat := range triageCategories {
		if strings.Contains(got, cat) {
			return cat
		}
	}
	return got
}

func percentile(xs []int64, p float64) int64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(p * float64(len(s)-1))
	return s[idx]
}

func fmtNS(ns int64) string {
	switch {
	case ns >= 1_000_000:
		return fmt.Sprintf("%.2fms", float64(ns)/1e6)
	case ns >= 1_000:
		return fmt.Sprintf("%.2fµs", float64(ns)/1e3)
	default:
		return fmt.Sprintf("%dns", ns)
	}
}
