package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// SupportCmd is the residual experiment: does the source SUPPORT the claim?
// Unlike `decompose` (verbatim presence, which a string tool nails), semantic
// support is the *uncovered residual* — a deterministic tool can't see that
// "cracks in the support beams" supports "structural problems." This is where
// decomposition is supposed to pay off, so it's the honest test of it.
//
// Conditions:
//   - opus-whole   : frontier baseline (the expensive "get it right inline")
//   - haiku-whole  : cheap model, whole task — does a cheap tier cover the residual?
//   - det-tool     : rg verbatim — should FAIL on paraphrase (proves the residual is real)
//   - haiku+rtv    : tool retrieves grounded sentences (cheap-LLM keyword → rg), cheap
//     model judges support from them — decomposition with a grounding gate
//
// Hard labels: supported (often via paraphrase) vs not (contradicted / unsupported).
type SupportCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Hard     bool   `help:"Use the HARD corpus: long buried-needle docs + subtle reasoning (quant traps, scope/negation, multi-hop, temporal) designed to separate cheap from frontier and give retrieval a needle."`
	Verbose  bool   `help:"Dump per-item kind, ground truth, each condition's verdict, retrieved evidence."`
}

type sClaim struct {
	src       string
	claim     string
	supported bool
	kind      string // verbatim | paraphrase | contradicted | unsupported
}

func sSources() map[string]string {
	return map[string]string{
		"bridge":  "The bridge was closed for repairs after an inspection found cracks in the support beams. Officials expect it to reopen next month.",
		"sales":   "Quarterly sales climbed 12 percent, driven by strong demand in overseas markets.",
		"vaccine": "In the study, participants who received the vaccine were half as likely to be hospitalized as those given a placebo.",
		"ceo":     "After the merger, Lena Ortiz stepped down as CEO and was succeeded by the former CFO.",
	}
}

func sClaims() []sClaim {
	return []sClaim{
		{"bridge", "The bridge was closed for repairs.", true, "verbatim"},
		{"bridge", "The bridge was shut down due to structural problems.", true, "paraphrase"},
		{"bridge", "The bridge collapsed during rush hour.", false, "unsupported"},
		{"sales", "Sales climbed 12 percent.", true, "verbatim"},
		{"sales", "Sales increased compared with earlier performance.", true, "paraphrase"},
		{"sales", "Sales declined this quarter.", false, "contradicted"},
		{"sales", "The company laid off staff this quarter.", false, "unsupported"},
		{"vaccine", "The vaccine reduced hospitalizations.", true, "paraphrase"},
		{"vaccine", "The vaccine eliminated all hospitalizations.", false, "contradicted"},
		{"vaccine", "The vaccine caused side effects.", false, "unsupported"},
		{"ceo", "Lena Ortiz is no longer the CEO.", true, "paraphrase"},
		{"ceo", "Lena Ortiz was promoted to CEO.", false, "contradicted"},
		{"ceo", "Lena Ortiz founded the company.", false, "unsupported"},
	}
}

func sSourcesHard() map[string]string {
	return map[string]string{
		"northwind":  "Northwind Logistics released its annual review on Monday. The company opened two new distribution centers in the Pacific Northwest during the first half of the year. Revenue from the freight division rose 8 percent, though the warehousing division saw margins compress under rising lease costs. Management noted that the new centers are not yet operating at full capacity. The board approved a share buyback of up to 200 million dollars. Chief Executive Mara Vance, who joined from a rival carrier in 2023, said the firm would prioritize automation over headcount growth. The report cautioned that a planned rail strike could disrupt deliveries in the third quarter. No dividend was declared.",
		"trial":      "The trial enrolled 1,200 adults with moderate hypertension across twelve clinical sites. Participants were randomized to receive either the experimental compound or a matching placebo for twenty-four weeks. The primary endpoint, a reduction in systolic blood pressure of at least 10 mmHg, was met in 58 percent of the treatment group versus 41 percent of controls. Adverse events were mostly mild, with headache reported more often in the treatment arm. The authors note the effect was smaller in participants over 65. A larger phase-three study is planned for next year. Funding was provided by the compound's manufacturer.",
		"succession": "Elena Ruiz served as the company's chief financial officer until March, when she was named chief executive. Her predecessor as CEO, Tom Park, retired after fifteen years.",
	}
}

func sClaimsHard() []sClaim {
	return []sClaim{
		{"northwind", "Northwind opened new facilities in the northwestern United States.", true, "buried-para"},
		{"northwind", "Northwind's CEO previously worked for a competitor.", true, "multihop-buried"},
		{"northwind", "Northwind's freight revenue grew by more than 10 percent.", false, "quant-trap(8<10)"},
		{"northwind", "Northwind plans to grow its workforce.", false, "scope/negation"},
		{"northwind", "Northwind declared a dividend and approved a buyback.", false, "partial-conjunction"},
		{"trial", "The drug lowered blood pressure more than placebo did.", true, "compare-nums"},
		{"trial", "A majority of the placebo group met the primary endpoint.", false, "quant-trap(41<50)"},
		{"trial", "The drug worked equally well in older participants.", false, "contradicted-buried"},
		{"trial", "The study was funded by the drug's maker.", true, "buried-para"},
		{"trial", "The drug cured hypertension in most patients.", false, "overclaim"},
		{"succession", "Elena Ruiz is the current CEO.", true, "temporal-multihop"},
		{"succession", "Tom Park is the current CFO.", false, "role-trap"},
		{"succession", "Elena Ruiz was the CFO before becoming CEO.", true, "temporal"},
	}
}

type supRow struct {
	idx                       int
	kind                      string
	supported                 bool
	opus, haiku, det, rtv     bool
	opusP, haikuP, rtvP       bool // parsed
	evidence                  string
	opusLat, haikuLat, rtvLat int64
}

func (c *SupportCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	tool, err := matchTool()
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	srcs, claims := sSources(), sClaims()
	if c.Hard {
		srcs, claims = sSourcesHard(), sClaimsHard()
	}

	var rows []supRow
	for i, cl := range claims {
		src := srcs[cl.src]
		opus, opusP, opusLat := judgeSupport(ctx, client, llm.ModelOpus, src, cl.claim)
		haiku, haikuP, haikuLat := judgeSupport(ctx, client, llm.ModelHaiku, src, cl.claim)
		det := detSupport(tool, src, cl.claim)
		ev, kw := retrieve(ctx, client, tool, src, cl.claim)
		rtv, rtvP, rtvLat := judgeSupportFrom(ctx, client, ev, cl.claim)
		rtvLat += kw // include keyword-pick latency

		rows = append(rows, supRow{
			idx: i, kind: cl.kind, supported: cl.supported,
			opus: opus, haiku: haiku, det: det, rtv: rtv,
			opusP: opusP, haikuP: haikuP, rtvP: rtvP,
			evidence: ev, opusLat: opusLat, haikuLat: haikuLat, rtvLat: rtvLat,
		})
	}
	supReport(rows, tool, c.Verbose)
	return nil
}

const supBar = `Reply with EXACTLY ONE WORD: SUPPORTED if the source semantically supports the claim (a paraphrase still counts), or UNSUPPORTED if the source does not support it, contradicts it, or is silent. Do not require verbatim wording.`

func judgeSupport(ctx context.Context, c *llm.Client, model, source, claim string) (bool, bool, int64) {
	r, err := c.Classify(ctx, model, "You judge whether a source supports a claim. "+supBar,
		fmt.Sprintf("SOURCE: %s\n\nCLAIM: %s", source, claim), 24)
	if err != nil {
		return false, false, 0
	}
	s, ok := parseSupported(r.Text)
	return s, ok, r.LatencyMS
}

func judgeSupportFrom(ctx context.Context, c *llm.Client, evidence, claim string) (bool, bool, int64) {
	src := evidence
	if strings.TrimSpace(src) == "" {
		src = "(no relevant evidence retrieved)"
	}
	r, err := c.Classify(ctx, llm.ModelHaiku, "You judge whether retrieved evidence supports a claim. "+supBar,
		fmt.Sprintf("EVIDENCE: %s\n\nCLAIM: %s", src, claim), 24)
	if err != nil {
		return false, false, 0
	}
	s, ok := parseSupported(r.Text)
	return s, ok, r.LatencyMS
}

// detSupport: the deterministic tool's only move is verbatim presence of the
// claim. It cannot see paraphrase — which is the point.
func detSupport(tool, source, claim string) bool {
	return toolMatch(tool, source, strings.TrimRight(claim, "."))
}

// retrieve: cheap model picks a keyword; the tool (rg) pulls matching sentences
// from the source. Returned evidence is grounded (verbatim sentences from source).
func retrieve(ctx context.Context, c *llm.Client, tool, source, claim string) (string, int64) {
	r, err := c.Classify(ctx, llm.ModelHaiku,
		`Output 1-3 key content words from the claim to search a source for (space-separated, no commentary).`, claim, 16)
	kw := ""
	var lat int64
	if err == nil {
		kw = strings.TrimSpace(r.Text)
		lat = r.LatencyMS
	}
	if kw == "" {
		return source, lat // fall back to whole source
	}
	// sentences, one per line, then rg each keyword.
	sentences := splitSentences(source)
	var hits []string
	seen := map[int]bool{}
	for _, w := range strings.Fields(kw) {
		for i, s := range sentences {
			if seen[i] {
				continue
			}
			if lineMatches(tool, s, w) {
				hits = append(hits, s)
				seen[i] = true
			}
		}
	}
	if len(hits) == 0 {
		return "", lat
	}
	return strings.Join(hits, " "), lat
}

func splitSentences(s string) []string {
	var out []string
	for _, part := range strings.SplitAfter(s, ". ") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lineMatches(tool, line, word string) bool {
	var cmd *exec.Cmd
	if tool == "rg" {
		cmd = exec.Command("rg", "-F", "-i", "-q", "--", word)
	} else {
		cmd = exec.Command("grep", "-F", "-i", "-q", "-e", word)
	}
	cmd.Stdin = strings.NewReader(line)
	return cmd.Run() == nil
}

func parseSupported(text string) (bool, bool) {
	up := strings.ToUpper(text)
	hasS, hasU := strings.Contains(up, "SUPPORTED"), strings.Contains(up, "UNSUPPORTED")
	// "UNSUPPORTED" contains "SUPPORTED" as a substring — check UNSUPPORTED first.
	switch {
	case hasU:
		return false, true
	case hasS:
		return true, true
	default:
		return false, false
	}
}

func supReport(rows []supRow, tool string, verbose bool) {
	n := len(rows)
	var opus, haiku, det, rtv confusion // positive = supported
	var opusLats, haikuLats, rtvLats []int64
	opusPF, haikuPF, rtvPF := 0, 0, 0 // parse-fails — EXCLUDED from accuracy, never defaulted
	// recall on SUPPORTED items (the residual a string tool can't paraphrase-match)
	paraTotal, opusPara, haikuPara, detPara, rtvPara := 0, 0, 0, 0, 0
	for _, r := range rows {
		// Only score PARSED verdicts — a parse-fail must never default to a class
		// that happens to match the label (the artifact this experiment first hit).
		if r.opusP {
			opus.add(r.opus, r.supported)
		} else {
			opusPF++
		}
		if r.haikuP {
			haiku.add(r.haiku, r.supported)
		} else {
			haikuPF++
		}
		det.add(r.det, r.supported) // deterministic, always parsed
		if r.rtvP {
			rtv.add(r.rtv, r.supported)
		} else {
			rtvPF++
		}
		opusLats = append(opusLats, r.opusLat)
		haikuLats = append(haikuLats, r.haikuLat)
		rtvLats = append(rtvLats, r.rtvLat)
		if r.supported {
			paraTotal++
			if r.opus {
				opusPara++
			}
			if r.haiku {
				haikuPara++
			}
			if r.det {
				detPara++
			}
			if r.rtv {
				rtvPara++
			}
		}
	}

	if verbose {
		fmt.Println("=== per-item (kind | want | opus/haiku/det/haiku+rtv) ===")
		for _, r := range rows {
			want := "no "
			if r.supported {
				want = "YES"
			}
			fmt.Printf("  %2d %-12s want=%s opus=%-3s haiku=%-3s det=%-3s rtv=%-3s | ev=%q\n",
				r.idx, r.kind, want, yn(r.opus, r.opusP), yn(r.haiku, r.haikuP), yn(r.det, true), yn(r.rtv, r.rtvP), truncate(r.evidence, 48))
		}
		fmt.Println()
	}

	fmt.Printf("population: N=%d · tool=%s · supported items=%d\n\n", n, tool, paraTotal)
	fmt.Println("=== semantic support — the uncovered residual (a string tool can't paraphrase-match) ===")
	fmt.Println("  (accuracy over PARSED verdicts only; parse-fails are excluded, never defaulted)")
	fmt.Printf("  opus-whole   acc=%.2f (n=%d)  median %d ms  parse-fail %d\n", opus.accuracy(), opus.n(), median(opusLats), opusPF)
	fmt.Printf("  haiku-whole  acc=%.2f (n=%d)  median %d ms  parse-fail %d\n", haiku.accuracy(), haiku.n(), median(haikuLats), haikuPF)
	fmt.Printf("  det-tool     acc=%.2f (n=%d)  ~0 ms\n", det.accuracy(), det.n())
	fmt.Printf("  haiku+rtv    acc=%.2f (n=%d)  median %d ms  parse-fail %d  (tool-retrieved grounded evidence)\n", rtv.accuracy(), rtv.n(), median(rtvLats), rtvPF)
	fmt.Printf("\n  recall on SUPPORTED (%d items — the residual):  opus %d  haiku %d  det %d  haiku+rtv %d\n",
		paraTotal, opusPara, haikuPara, detPara, rtvPara)

	fmt.Println("\nThe residual is real if det-tool misses the paraphrase items while the models catch")
	fmt.Println("them. The shift-left question: does the CHEAP model (haiku) cover the residual ~as well")
	fmt.Println("as opus? If yes, shift to haiku; the frontier isn't needed here. Verify --verbose first.")
}

func yn(v, parsed bool) string {
	if !parsed {
		return "??"
	}
	if v {
		return "yes"
	}
	return "no"
}
