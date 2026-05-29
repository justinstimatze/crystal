package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// ExperimentCmd runs the first LIVE grounding of the corrected thesis on a
// verifiable chore (structured extraction). It measures three numbers the
// lattice could only assume:
//
//  1. Substitution fidelity per tier (Opus/Sonnet/Haiku vs exact gold) — the
//     grounded "cheapest sufficient tier", correcting the binary-measure error.
//  2. Fuzzy-channel λ — how often a supervisor's verdict from a COMPRESSED
//     summary disagrees with its verdict from the FULL worker output.
//  3. Guardrail coverage g — fraction of worker errors a DETERMINISTIC,
//     lossless schema/validity check catches with no model call at all.
//
// Limitation (stated, not hidden): the items are synthetic-but-objective, not
// drawn from the substrate, and the verdict judge is itself an LLM. This is a
// first grounding, not the final word.
type ExperimentCmd struct {
	CacheDir  string  `help:"Disk cache dir." default:".crystal-cache"`
	Threshold float64 `help:"Substitution fidelity needed to call a tier 'sufficient'." default:"0.95"`
}

type item struct{ Text, Name, Role, Org string }

func items() []item {
	return []item{
		{"Jane Smith is the CEO of Acme Corp.", "jane smith", "ceo", "acme corp"},
		{"Dr. Raj Patel serves as Chief Medical Officer at Mercy Health.", "raj patel", "chief medical officer", "mercy health"},
		{"The startup Nimbus was co-founded by Lena Ortiz, who acts as CTO.", "lena ortiz", "cto", "nimbus"},
		{"At Globex, Tomás Vega holds the title of VP of Engineering.", "tomás vega", "vp of engineering", "globex"},
		{"Priya Nair, the head of design, recently joined Foldspace.", "priya nair", "head of design", "foldspace"},
		{"Marcus Lee runs operations as COO for the logistics firm Haulwell.", "marcus lee", "coo", "haulwell"},
		{"Formerly at IBM, Anna Kowalski is now principal engineer at Vertex Labs.", "anna kowalski", "principal engineer", "vertex labs"},
		{"The board appointed Chen Wei as interim CFO of BluePeak Industries.", "chen wei", "interim cfo", "bluepeak industries"},
		{"Sofia Russo leads the data science team at the fintech Coyne & Park.", "sofia russo", "data science lead", "coyne & park"},
		{"Following the merger, Omar Haddad became regional director for EMEA at Northstar.", "omar haddad", "regional director", "northstar"},
	}
}

const extractSys = `Extract the person from the text. Reply with ONLY a JSON object, no prose:
{"name": "...", "role": "...", "org": "..."}
Use the role title as stated. Use the organization as named.`

type extraction struct {
	Name string `json:"name"`
	Role string `json:"role"`
	Org  string `json:"org"`
}

func (c *ExperimentCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	its := items()
	tiers := []struct{ name, id string }{{"opus", llm.ModelOpus}, {"sonnet", llm.ModelSonnet}, {"haiku", llm.ModelHaiku}}

	// 1. Substitution fidelity per tier + capture haiku's raw outputs for the
	//    channel/guardrail experiments.
	fmt.Println("=== 1. substitution fidelity (vs exact gold) ===")
	var haikuOut []extraction
	var haikuValid []bool
	for _, t := range tiers {
		correct := 0
		for _, it := range its {
			ex, raw, valid := extractOne(ctx, client, t.id, it.Text)
			if t.name == "haiku" {
				haikuOut = append(haikuOut, ex)
				haikuValid = append(haikuValid, valid)
				_ = raw
			}
			if scoreExact(ex, it) {
				correct++
			}
		}
		fid := float64(correct) / float64(len(its))
		fmt.Printf("  %-7s %d/%d = %.2f%s\n", t.name, correct, len(its), fid, sufficientMark(fid, c.Threshold))
	}

	// 2. Guardrail coverage g — a DETERMINISTIC check (valid JSON, all fields
	//    non-empty) over the worker (haiku). g = errors it catches / total errors.
	fmt.Println("\n=== 2. guardrail coverage g (deterministic, lossless, no API) ===")
	totalErr, caught := 0, 0
	for i, it := range its {
		wrong := !scoreExact(haikuOut[i], it)
		if wrong {
			totalErr++
			if !haikuValid[i] || haikuOut[i].Name == "" || haikuOut[i].Role == "" || haikuOut[i].Org == "" {
				caught++ // a schema/validity guardrail flags this error losslessly
			}
		}
	}
	g := 0.0
	if totalErr > 0 {
		g = float64(caught) / float64(totalErr)
	}
	fmt.Printf("  worker(haiku) errors=%d, caught by deterministic schema guardrail=%d → g=%.2f\n", totalErr, caught, g)
	fmt.Println("  (errors a schema check can't catch are the irreducible fuzzy residual — need a richer guardrail or the fuzzy channel)")

	// 3. Fuzzy-channel λ — supervisor (opus) verdict from FULL worker output vs
	//    from a COMPRESSED summary. λ = disagreement.
	fmt.Println("\n=== 3. fuzzy-channel λ (opus verdict: full signal vs compressed summary) ===")
	disagree := 0
	for i, it := range its {
		full := verdictFull(ctx, client, it.Text, haikuOut[i])
		summary := summarize(ctx, client, it.Text, haikuOut[i])
		lossy := verdictSummary(ctx, client, summary)
		if full != lossy {
			disagree++
		}
	}
	lambda := float64(disagree) / float64(len(its))
	fmt.Printf("  verdict disagreement (full vs summary) = %d/%d → measured λ=%.2f\n", disagree, len(its), lambda)
	fmt.Println("\nfeeds the lattice: real g and λ replace the assumed knobs. A structured/guardrail")
	fmt.Println("channel (g>0) is lossless for what it covers; λ is the residual fuzzy loss.")
	return nil
}

func extractOne(ctx context.Context, c *llm.Client, model, text string) (extraction, string, bool) {
	r, err := c.Complete(ctx, model, extractSys, text, 120)
	if err != nil {
		return extraction{}, "", false
	}
	ex, ok := parseExtraction(r.Text)
	return ex, r.Text, ok
}

func parseExtraction(s string) (extraction, bool) {
	a, b := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if a < 0 || b <= a {
		return extraction{}, false
	}
	var ex extraction
	if json.Unmarshal([]byte(s[a:b+1]), &ex) != nil {
		return extraction{}, false
	}
	return ex, true
}

func norm(s string) string { return strings.TrimSpace(strings.ToLower(s)) }

func scoreExact(ex extraction, it item) bool {
	return norm(ex.Name) == it.Name && norm(ex.Role) == it.Role && norm(ex.Org) == it.Org
}

func verdictFull(ctx context.Context, c *llm.Client, text string, ex extraction) bool {
	sys := `You verify an extraction. Reply ONLY "YES" if the extraction is fully correct for the text, else "NO".`
	p := fmt.Sprintf("Text: %s\nExtraction: name=%q role=%q org=%q", text, ex.Name, ex.Role, ex.Org)
	return yes(c.Complete(ctx, llm.ModelOpus, sys, p, 5))
}

func summarize(ctx context.Context, c *llm.Client, text string, ex extraction) string {
	sys := `Summarize, in 12 words or fewer, what the worker extracted and whether it looks right. No JSON.`
	p := fmt.Sprintf("Text: %s\nWorker extracted: name=%q role=%q org=%q", text, ex.Name, ex.Role, ex.Org)
	r, _ := c.Complete(ctx, llm.ModelHaiku, sys, p, 40)
	return r.Text
}

func verdictSummary(ctx context.Context, c *llm.Client, summary string) bool {
	sys := `A subordinate reported on a worker's extraction. Based ONLY on this report, reply "YES" if the worker was correct, else "NO".`
	return yes(c.Complete(ctx, llm.ModelOpus, sys, "Report: "+summary, 5))
}

func yes(r llm.Result, err error) bool {
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(r.Text)), "YES")
}

func sufficientMark(fid, threshold float64) string {
	if fid >= threshold {
		return "  ← sufficient"
	}
	return ""
}
