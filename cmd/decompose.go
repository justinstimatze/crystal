package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// DecomposeCmd is the A4 experiment: does *decomposing* a chore — cheap model
// driving a robust CLI tool — beat shifting the WHOLE chore to the cheap model?
// Chore: quote verification (is this claimed quote actually present in the
// source?), a verification-shaped task that should decompose well.
//
// Three conditions, hard labels by construction:
//   - whole-haiku : Haiku judges presence from source+quote (the fuzzy whole-task shift)
//   - det-tool    : `rg -F -i` (the modern tool weir prefers) matches the full quote — no model
//   - haiku+tool  : Haiku picks a distinctive literal fragment, `rg` decides presence
//
// The dangerous category is *fabricated-plausible* quotes (a digit flipped, or the
// real Tolstoy line when the source twisted it) — exactly where a fuzzy reader
// hallucinates "present" and a string tool correctly says "absent."
type DecomposeCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool   `help:"Dump per-item category, ground truth, and each condition's verdict."`
}

type qSource struct{ id, text string }
type qClaim struct {
	src      string
	quote    string
	present  bool   // ground truth
	category string // verbatim | variant | absent | fabricated
}

func qSources() map[string]string {
	return map[string]string{
		"zoning":  "The committee voted 7 to 2 on Tuesday to approve the new zoning ordinance for the riverside district.",
		"trial":   "Dr. Elena Park said the trial showed a 34% reduction in symptoms among participants over twelve weeks.",
		"revenue": "Our Q3 revenue grew to $4.2 million, driven primarily by the enterprise segment in the Asia-Pacific region.",
		"tolstoy": "The novel opens with the line, 'All happy families resemble a delusion,' a deliberate inversion of Tolstoy.",
	}
}

func qClaims() []qClaim {
	return []qClaim{
		{"zoning", "voted 7 to 2 on Tuesday", true, "verbatim"},
		{"zoning", "Voted 7 To 2 On Tuesday", true, "variant"},
		{"zoning", "voted 8 to 1 on Tuesday", false, "fabricated"},
		{"zoning", "approved the budget for the new stadium", false, "absent"},
		{"trial", "34% reduction in symptoms", true, "verbatim"},
		{"trial", "43% reduction in symptoms", false, "fabricated"},
		{"trial", "no significant effect was observed", false, "absent"},
		{"revenue", "Q3 revenue grew to $4.2 million", true, "verbatim"},
		{"revenue", "q3 revenue grew to $4.2 million", true, "variant"},
		{"revenue", "Q3 revenue grew to $4.2 billion", false, "fabricated"},
		{"tolstoy", "All happy families resemble a delusion", true, "verbatim"},
		{"tolstoy", "All happy families are alike", false, "fabricated"},
		{"tolstoy", "It was the best of times", false, "absent"},
	}
}

type decRow struct {
	idx                int
	category           string
	present            bool
	whole, det, hktool bool
	wholeParsed        bool
	fragment           string
	wholeLat, hkLat    int64
}

func (c *DecomposeCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	tool, err := matchTool()
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	srcs := qSources()

	var rows []decRow
	for i, cl := range qClaims() {
		src := srcs[cl.src]

		whole, wholeParsed, wholeLat := wholeTaskPresent(ctx, client, src, cl.quote)
		det := toolMatch(tool, src, cl.quote)
		frag, hkLat := pickFragment(ctx, client, cl.quote)
		hktool := toolMatch(tool, src, frag)

		rows = append(rows, decRow{
			idx: i, category: cl.category, present: cl.present,
			whole: whole, det: det, hktool: hktool, wholeParsed: wholeParsed,
			fragment: frag, wholeLat: wholeLat, hkLat: hkLat,
		})
	}
	decReport(rows, tool, c.Verbose)
	return nil
}

// --- the robust CLI tool (weir's preference: rg over grep) ---

func matchTool() (string, error) {
	for _, t := range []string{"rg", "grep"} {
		if _, err := exec.LookPath(t); err == nil {
			return t, nil
		}
	}
	return "", fmt.Errorf("neither rg nor grep on PATH — this experiment needs a string-search tool")
}

// toolMatch reports whether pattern occurs in source, case-insensitive, fixed-string.
func toolMatch(tool, source, pattern string) bool {
	var cmd *exec.Cmd
	switch tool {
	case "rg":
		cmd = exec.Command("rg", "-F", "-i", "-q", "--", pattern)
	default:
		cmd = exec.Command("grep", "-F", "-i", "-q", "-e", pattern)
	}
	cmd.Stdin = strings.NewReader(source)
	return cmd.Run() == nil // exit 0 = match
}

// --- the cheap model's two roles ---

func wholeTaskPresent(ctx context.Context, c *llm.Client, source, quote string) (present, parsed bool, lat int64) {
	sys := `Is the quoted text present in the source — verbatim, ignoring only case and whitespace? Reply with EXACTLY ONE WORD: PRESENT or ABSENT. A paraphrase or any changed word/number is ABSENT.`
	p := fmt.Sprintf("SOURCE: %s\n\nQUOTE: %s", source, quote)
	r, err := c.Classify(ctx, llm.ModelHaiku, sys, p, 8)
	if err != nil {
		return false, false, 0
	}
	pr, ok := parsePresent(r.Text)
	return pr, ok, r.LatencyMS
}

func pickFragment(ctx context.Context, c *llm.Client, quote string) (string, int64) {
	sys := `Copy a distinctive run of 4 to 8 consecutive words from the text below, verbatim. Output ONLY those words — no quotation marks, no explanation, no refusal, no preamble. If the text is shorter than 8 words, output all of it unchanged.`
	r, err := c.Classify(ctx, llm.ModelHaiku, sys, quote, 30)
	if err != nil {
		return quote, 0
	}
	frag := strings.TrimSpace(strings.Trim(r.Text, `"'`))
	if frag == "" {
		frag = quote
	}
	return frag, r.LatencyMS
}

func parsePresent(text string) (bool, bool) {
	up := strings.ToUpper(text)
	hasP, hasA := strings.Contains(up, "PRESENT"), strings.Contains(up, "ABSENT")
	switch {
	case hasP && !hasA:
		return true, true
	case hasA && !hasP:
		return false, true
	default:
		return false, false
	}
}

func decReport(rows []decRow, tool string, verbose bool) {
	n := len(rows)
	var whole, det, hktool confusion // positive class = "present"
	wholeParseFail := 0
	var wholeLats, hkLats []int64
	// hallucination = predicted present on a NOT-present (absent/fabricated) item
	wholeHall, detHall, hkHall := 0, 0, 0
	for _, r := range rows {
		whole.add(r.whole, r.present)
		det.add(r.det, r.present)
		hktool.add(r.hktool, r.present)
		if !r.wholeParsed {
			wholeParseFail++
		}
		if !r.present {
			if r.whole {
				wholeHall++
			}
			if r.det {
				detHall++
			}
			if r.hktool {
				hkHall++
			}
		}
		wholeLats = append(wholeLats, r.wholeLat)
		hkLats = append(hkLats, r.hkLat)
	}

	if verbose {
		fmt.Println("=== per-item (truth | whole-haiku / det-tool / haiku+tool) ===")
		for _, r := range rows {
			truth := "ABSENT "
			if r.present {
				truth = "PRESENT"
			}
			fmt.Printf("  %2d %-10s want=%s  whole=%-6s det=%-6s hk+tool=%-6s  frag=%q\n",
				r.idx, r.category, truth, present(r.whole, r.wholeParsed), present(r.det, true), present(r.hktool, true), truncate(r.fragment, 40))
		}
		fmt.Println()
	}

	notPresent := 0
	for _, r := range rows {
		if !r.present {
			notPresent++
		}
	}

	fmt.Printf("population: N=%d (%d present, %d not) · tool=%s\n\n", n, n-notPresent, notPresent, tool)
	fmt.Println("=== quote verification — whole-task model vs decomposed (model + robust tool) ===")
	fmt.Printf("  whole-haiku  acc=%.2f  median latency %d ms  hallucinated-present %d/%d  (parse-fail %d)\n",
		whole.accuracy(), median(wholeLats), wholeHall, notPresent, wholeParseFail)
	fmt.Printf("  det-tool     acc=%.2f  ~0 ms (no model)       hallucinated-present %d/%d\n",
		det.accuracy(), detHall, notPresent)
	fmt.Printf("  haiku+tool   acc=%.2f  median latency %d ms  hallucinated-present %d/%d\n",
		hktool.accuracy(), median(hkLats), hkHall, notPresent)

	fmt.Println("\nThesis: for a verification-shaped chore the deterministic tool nails the mechanical")
	fmt.Println("matching at ~0ms while the whole-task model is slower and can hallucinate 'present' on")
	fmt.Println("fabricated-but-plausible quotes. The model earns its keep only on the fuzzy residual")
	fmt.Println("(paraphrase/semantic presence) — not tested here. Verify --verbose rows before trusting.")
}

func present(v, parsed bool) string {
	if !parsed {
		return "??"
	}
	if v {
		return "present"
	}
	return "absent"
}
