package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
	"github.com/justinstimatze/crystal/internal/publicai"
)

// TransferCmd MEASURES the recipe ladder instead of asserting it (docs/
// RECIPE_LADDER.md). The claim under test: recipe-specificity sets the executor
// floor — a weaker model takes a GROWING fraction of a chore as Opus's recipe
// gets more specific. So we sweep recipe-rung × executor-tier and report the
// TRANSFER FRACTION (how often the weak executor + recipe reproduces the golden)
// at each cell.
//
// Chore: entity-spec → Go struct (reuses the demo's golden infra). Rungs:
//
//	none        — the weak model with NO recipe (baseline floor)
//	plan        — Opus authors a one-sentence high-level description
//	recipe      — Opus authors step-by-step prose
//	pseudocode  — Opus authors near-mechanical pseudocode
//	code        — the deterministic template (NO model; 100% by construction)
//
// Executors: the weak cloud model (Haiku) runs none/plan/recipe/pseudocode; the
// code rung is deterministic; Opus-direct is the reference ceiling. The local
// open-weight tier is the deferred cell (away from the 3080).
//
// The thing to read off: does Haiku's transfer fraction RISE with rung
// specificity? If yes, the ladder is real and measured. If flat, the chore
// doesn't discriminate rungs (an honest negative — pick a harder chore).
type TransferCmd struct {
	Spec      string `help:"Entity spec (the recurring chore)." default:"testdata/demo/entities.json"`
	FirstK    int    `help:"Worked examples shown to Opus to author each rung's recipe." default:"5"`
	CacheDir  string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	OpusModel string `help:"The expensive tier that authors the recipes (and the ceiling)." default:"claude-opus-4-8"`
	WeakModel string `help:"The weak executor that follows the recipes." default:"claude-haiku-4-5"`
	OpenModel string `help:"Open-weight executor id served by an OpenAI-compatible endpoint (fills the local-open cell). Empty ⇒ that cell stays deferred. Endpoint from MODAL_VLLM_URL + MODAL_VLLM_KEY." default:""`
	Verbose   bool   `help:"Print each authored recipe and per-entity outcomes."`
}

type rung struct {
	name   string
	prompt string // instruction to Opus for authoring this rung's recipe
}

func (c *TransferCmd) Run() error {
	spec, err := loadSpec(c.Spec)
	if err != nil {
		return usageError{err}
	}
	if len(spec.Units) < c.FirstK+2 {
		return usageError{fmt.Errorf("spec has %d units; need ≥ first-k+2", len(spec.Units))}
	}
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	// Optional local-open executor: an open-weight model behind an OpenAI-
	// compatible endpoint (a Modal-hosted vLLM ~32B, the reliable stand-in for
	// the house 3080 that spills a 35B). Constructed up front so a misconfigured
	// endpoint fails before any paid Opus authoring. llm.New already loaded .env.
	var openClient *publicai.Client
	if c.OpenModel != "" {
		url, key := os.Getenv("MODAL_VLLM_URL"), os.Getenv("MODAL_VLLM_KEY")
		if url == "" || key == "" {
			return usageError{fmt.Errorf("--open-model set but MODAL_VLLM_URL / MODAL_VLLM_KEY missing from env or .env")}
		}
		if openClient, err = publicai.NewAt(c.CacheDir, url, key); err != nil {
			return usageError{err}
		}
	}
	ctx := context.Background()
	worked := spec.Units[:c.FirstK]
	test := spec.Units[c.FirstK:]

	fmt.Printf("transfer: measure the recipe ladder — does the weak executor (%s) take more of the\n", c.WeakModel)
	fmt.Printf("chore as Opus's recipe gets more specific? Chore: entity→Go struct, %d test instances.\n\n", len(test))

	// ---- Opus authors a recipe at each rung from the worked examples ----
	rungs := []rung{
		{"plan", "Write a ONE-SENTENCE, high-level description of how to turn an entity spec into the Go struct shown. No steps, no syntax, no code."},
		{"recipe", "Write STEP-BY-STEP numbered PROSE instructions for turning an entity spec into the Go struct shown, including the exact json-tag convention. No code blocks."},
		{"pseudocode", "Write precise PSEUDOCODE for turning an entity spec into the Go struct shown — spell out the exact output format, field order, and json-tag rendering. Near-mechanical, but not real Go."},
	}
	examples := workedExamplesBlock(worked)
	recipes := map[string]string{}
	fmt.Println("=== Opus authors a recipe at each rung (from the worked examples) ===")
	for _, rg := range rungs {
		sys := "You are writing a reusable instruction for a WEAKER model to follow. " + rg.prompt +
			" Output ONLY the instruction text."
		r, err := client.Complete(ctx, c.OpusModel, sys, examples, 2048)
		if err != nil {
			return usageError{fmt.Errorf("authoring %s: %w", rg.name, err)}
		}
		recipes[rg.name] = stripFence(r.Text)
		fmt.Printf("  %-11s authored (%d chars)\n", rg.name, len(recipes[rg.name]))
		if c.Verbose {
			fmt.Printf("    %s\n", indentLines(recipes[rg.name], "    "))
		}
	}
	fmt.Println()

	// ---- run the executor sweep; tally transfer fraction (golden-correct / N) ----
	type result struct {
		label    string
		executor string
		correct  int
	}
	var results []result

	// none: weak model, no recipe (the baseline floor)
	results = append(results, result{"none (no recipe)", c.WeakModel,
		c.runWeak(ctx, client, test, "")})
	// plan / recipe / pseudocode: weak model + Opus's recipe
	for _, rg := range rungs {
		results = append(results, result{rg.name, c.WeakModel,
			c.runWeak(ctx, client, test, recipes[rg.name])})
	}
	// code: deterministic template (no model)
	detCorrect := 0
	for _, u := range test {
		if matchesGolden(goldenOf(u), u) {
			detCorrect++
		}
	}
	results = append(results, result{"code (template)", "deterministic", detCorrect})
	// ceiling: Opus direct, no recipe
	ceiling := c.runStrong(ctx, client, test)

	// local-open executor: the same rungs run through the open-weight model
	// (fills the previously-deferred cell) when --open-model is set.
	var openResults []result
	if openClient != nil {
		openResults = append(openResults, result{"none (no recipe)", c.OpenModel,
			c.runOpen(ctx, openClient, test, "")})
		for _, rg := range rungs {
			openResults = append(openResults, result{rg.name, c.OpenModel,
				c.runOpen(ctx, openClient, test, recipes[rg.name])})
		}
	}

	// ---- report ----
	n := len(test)
	fmt.Println("=== transfer fraction: weak executor following Opus's recipe at each rung ===")
	fmt.Printf("  %-18s %-14s %s\n", "rung", "executor", "transfer (golden-correct / N)")
	for _, r := range results {
		fmt.Printf("  %-18s %-14s %d/%d = %.0f%%\n", r.label, r.executor, r.correct, n, 100*float64(r.correct)/float64(n))
	}
	fmt.Println("  ─── reference ───")
	fmt.Printf("  %-18s %-14s %d/%d = %.0f%%   (ceiling)\n", "direct", c.OpusModel, ceiling, n, 100*float64(ceiling)/float64(n))
	if openClient == nil {
		fmt.Printf("  %-18s %-14s (deferred — pass --open-model to fill this cell)\n", "any rung", "local-open")
	}
	fmt.Println()

	// ---- local-open block: the recipe ladder on the open-weight tier ----
	if openClient != nil {
		fmt.Println("=== local-open: the same ladder on an open-weight model (Modal-hosted vLLM) ===")
		fmt.Printf("  %-18s %-14s %s\n", "rung", "executor", "transfer (golden-correct / N)")
		for _, r := range openResults {
			fmt.Printf("  %-18s %-14s %d/%d = %.0f%%\n", r.label, shortModel(r.executor), r.correct, n, 100*float64(r.correct)/float64(n))
		}
		// Read the open cell honestly against the closed cheap tier (Haiku) — the
		// point of filling this cell is "does an OPEN model reach the closed cheap
		// tier, and does the recipe ladder hold there too?"
		openNone := openResults[0].correct
		openBest, openMonotone := openNone, true
		for i, r := range openResults[1:] {
			if r.correct > openBest {
				openBest = r.correct
			}
			if r.correct < openResults[i].correct { // strictly below the previous rung
				openMonotone = false
			}
		}
		haikuBest := results[0].correct
		for _, r := range results[1:4] { // plan/recipe/pseudocode
			if r.correct > haikuBest {
				haikuBest = r.correct
			}
		}
		fmt.Println("  ─── reading (n small — suggestive) ───")
		switch {
		case openBest >= ceiling:
			fmt.Printf("  The open model's best rung (%d/%d) MEETS the Opus-direct ceiling (%d/%d) — the open tier is\n", openBest, n, ceiling, n)
			fmt.Printf("    at the frontier's fidelity on this chore, at a fraction of the cost and with no closed weights.\n")
		case openBest >= haikuBest:
			fmt.Printf("  The open model's best rung (%d/%d) TIES the closed cheap tier's best (Haiku %d/%d) — an open,\n", openBest, n, haikuBest, n)
			fmt.Printf("    self-hostable model reaches the paid-closed cheap tier once given the specific recipe.\n")
		default:
			fmt.Printf("  The open model's best rung (%d/%d) stays below the closed cheap tier's best (Haiku %d/%d) —\n", openBest, n, haikuBest, n)
			fmt.Printf("    the open tier does not yet reach the closed cheap tier on this chore (an honest gap).\n")
		}
		if openMonotone && openBest > openNone {
			fmt.Printf("  The recipe ladder holds MONOTONICALLY on the open model (%d→…→%d): more specificity, more\n", openNone, openBest)
			fmt.Printf("    transfer, no dip — the open tier tracks the ladder crisply even where Haiku wobbles at a rung.\n")
		}
		fmt.Println()
	}

	// ---- read the curve (honestly — surface non-monotonicity, don't smooth it) ----
	none, plan, recipe, pseudo, code := results[0].correct, results[1].correct, results[2].correct, results[3].correct, results[4].correct
	best := none
	for _, r := range results[1:4] {
		if r.correct > best {
			best = r.correct
		}
	}
	fmt.Println("=== reading (n is small — suggestive, not conclusive) ===")
	if best > none {
		fmt.Printf("  Directional support: the weak model's best recipe rung (%d/%d) beats no-recipe (%d/%d) — a\n", best, n, none, n)
		fmt.Printf("  more specific recipe lets the weak executor take more, and the deterministic code rung (%d/%d)\n", code, n)
		fmt.Printf("  needs no model at all. Opus authored the recipe once; %s executed it per instance.\n", c.WeakModel)
	} else {
		fmt.Printf("  Flat/negative: the best recipe rung (%d/%d) does not beat no-recipe (%d/%d). This chore does\n", best, n, none, n)
		fmt.Printf("  not discriminate rungs for this executor — pick a harder one (weak model failing at the vague end).\n")
	}
	// The non-monotonicity is the most interesting result — flag it loudly.
	if plan < none {
		fmt.Printf("  ⚠ NON-MONOTONIC: the vague 'plan' rung (%d/%d) scored WORSE than no recipe (%d/%d). A bad\n", plan, n, none, n)
		fmt.Printf("    abstraction level HURTS — a vague recipe misleads the weak model more than silence. The ladder\n")
		fmt.Printf("    is not 'more words = better'; it is 'the right specificity', and the wrong rung is below the floor.\n")
	}
	if pseudo <= recipe {
		fmt.Printf("  Plateau: 'pseudocode' (%d/%d) did not beat 'recipe' (%d/%d) — extra specificity stopped paying off\n", pseudo, n, recipe, n)
		fmt.Printf("    before reaching code. The residual past the plateau is what only the deterministic rung closes.\n")
	}
	if code > ceiling {
		fmt.Printf("  Note: the deterministic code rung (%d/%d) BEAT Opus-direct (%d/%d) — the generator IS the golden\n", code, n, ceiling, n)
		fmt.Printf("    by construction, so it is exact where even the frontier diverges on a strict gate. Cheapest AND most accurate.\n")
	}
	return nil
}

// runWeak executes the chore with the weak model under an optional recipe and
// returns the count of golden-correct outputs.
func (c *TransferCmd) runWeak(ctx context.Context, client *llm.Client, test []Unit, recipe string) int {
	correct := 0
	for _, u := range test {
		sys := "Produce a Go struct type declaration from the entity spec. Output ONLY the Go code — no prose, no markdown fences, no comments."
		prompt := ""
		if recipe != "" {
			sys = "Follow the instruction to produce a Go struct type from the entity spec. Output ONLY the Go code — no prose, no markdown fences, no comments."
			prompt = "Instruction:\n" + recipe + "\n\n"
		}
		prompt += "Entity spec (JSON):\n" + mustJSON(u) + "\n\nGo struct:"
		r, err := client.Complete(ctx, c.WeakModel, sys, prompt, 1024)
		if err != nil {
			continue
		}
		if matchesGolden(stripFence(r.Text), u) {
			correct++
		}
	}
	return correct
}

// runOpen executes the chore with the open-weight model (via an OpenAI-
// compatible endpoint) under an optional recipe. It mirrors runWeak exactly so
// the only variable across the two executors is the model — a fair rung-for-rung
// comparison of the closed weak tier (Haiku) against the open local tier.
func (c *TransferCmd) runOpen(ctx context.Context, oc *publicai.Client, test []Unit, recipe string) int {
	correct := 0
	for _, u := range test {
		sys := "Produce a Go struct type declaration from the entity spec. Output ONLY the Go code — no prose, no markdown fences, no comments."
		prompt := ""
		if recipe != "" {
			sys = "Follow the instruction to produce a Go struct type from the entity spec. Output ONLY the Go code — no prose, no markdown fences, no comments."
			prompt = "Instruction:\n" + recipe + "\n\n"
		}
		prompt += "Entity spec (JSON):\n" + mustJSON(u) + "\n\nGo struct:"
		r, err := oc.Classify(ctx, c.OpenModel, sys, prompt, 1024)
		if err != nil {
			continue
		}
		if matchesGolden(stripFence(r.Text), u) {
			correct++
		}
	}
	return correct
}

// shortModel trims an HF-style model path to its final component for the report
// (e.g. "Qwen/Qwen2.5-32B-Instruct" → "Qwen2.5-32B-Instruct").
func shortModel(m string) string {
	if i := strings.LastIndexByte(m, '/'); i >= 0 && i+1 < len(m) {
		return m[i+1:]
	}
	return m
}

// runStrong is the Opus-direct ceiling (no recipe).
func (c *TransferCmd) runStrong(ctx context.Context, client *llm.Client, test []Unit) int {
	correct := 0
	for _, u := range test {
		sys := "Produce a Go struct type declaration from the entity spec. Output ONLY the Go code — no prose, no markdown fences, no comments."
		r, err := client.Complete(ctx, c.OpusModel, sys, "Entity spec (JSON):\n"+mustJSON(u)+"\n\nGo struct:", 1024)
		if err != nil {
			continue
		}
		if matchesGolden(stripFence(r.Text), u) {
			correct++
		}
	}
	return correct
}

// workedExamplesBlock renders the (spec → golden) pairs Opus authors recipes from.
func workedExamplesBlock(worked []Unit) string {
	var b string
	b = "Worked examples — each an entity spec (JSON) and the exact Go struct it must produce:\n\n"
	for _, u := range worked {
		b += mustJSON(u) + "\n=>\n" + goldenOf(u) + "\n"
	}
	return b
}
