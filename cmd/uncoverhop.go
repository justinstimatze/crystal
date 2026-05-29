package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// UncoverHopCmd is the second grounding hop — the half of STACKING #7 that
// ground-hop could not reach. ground-hop showed that for byte-exact drift the
// deterministic comparator covers everything (g=1), leaving no residual for a
// fuzzy channel to earn its keep on. This command constructs a regime where a
// deterministic check PROVABLY cannot catch some real errors (g<1), then asks:
// does the fuzzy channel recover the uncovered residual, and how much
// catching-power is lost across one prose-compression hop?
//
// Task: extract {name, role, org} from a sentence. Errors are INJECTED (hard
// labels by construction — the lesson from EXPERIMENT/GROUNDHOP), in two tiers
// by what a deterministic check can catch:
//
//	tier-1 — a field is swapped to an UNRELATED value not present in the source.
//	         A substring-grounding check catches it (the value isn't in the text).
//	tier-2 — a field is swapped to a DISTRACTOR that DOES appear in the source
//	         (e.g. a person's former org the sentence still names). It survives
//	         the substring check — only a semantic (fuzzy) judge can catch it.
//
// Two deterministic guardrails make the "checkable fraction" visible:
//
//	det-schema   — valid, non-empty JSON. Passes everything → g≈0.
//	det-grounded — every extracted field must be a substring of the source.
//	               Catches tier-1, MISSES tier-2.
//
// The grounded per-hop loss λ is measured on the UNCOVERED residual (tier-2):
// R_full (supervisor recall with the full source) minus R_prose (recall from
// only a prose diff). This λ is scored against HARD LABELS, never against a
// fallible model verdict (the contaminated metric ground-hop exposed).
type UncoverHopCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool   `help:"Dump per-item tiers, channel verdicts, and prose summaries."`
}

// exItem is a source sentence with its gold extraction and a tier-2 distractor:
// DValue is a plausible WRONG value for field DField that nonetheless appears
// verbatim in Text (so a substring-grounding check cannot catch it).
type exItem struct {
	Text, Name, Role, Org string
	DField, DValue        string
}

func exItems() []exItem {
	return []exItem{
		{"Formerly a manager at IBM, Anna Kowalski now works as principal engineer at Vertex Labs.", "Anna Kowalski", "principal engineer", "Vertex Labs", "org", "IBM"},
		{"Lena Ortiz, once a designer at Adobe, co-founded Nimbus and serves as its CTO.", "Lena Ortiz", "CTO", "Nimbus", "org", "Adobe"},
		{"Reporting to outgoing COO Dana Pike, Marcus Lee was named the new COO of Haulwell.", "Marcus Lee", "COO", "Haulwell", "name", "Dana Pike"},
		{"After her CFO stint at BluePeak, Chen Wei joined Northstar as its chief executive officer.", "Chen Wei", "chief executive officer", "Northstar", "role", "CFO"},
		{"Priya Nair left her role as head of design at Foldspace to become VP of product at Lumen.", "Priya Nair", "VP of product", "Lumen", "org", "Foldspace"},
		{"Tomás Vega, brother of Globex founder Mara Vega, is VP of Engineering at Globex.", "Tomás Vega", "VP of Engineering", "Globex", "name", "Mara Vega"},
		{"Before Mercy Health recruited him, Raj Patel was chief resident at St. Luke's; he is now their Chief Medical Officer.", "Raj Patel", "Chief Medical Officer", "Mercy Health", "org", "St. Luke's"},
		{"Jane Smith succeeded interim CEO Paul Adams as the permanent CEO of Acme Corp.", "Jane Smith", "CEO", "Acme Corp", "name", "Paul Adams"},
		{"Sofia Russo moved from her analyst job to become head of data science at Coyne & Park.", "Sofia Russo", "head of data science", "Coyne & Park", "role", "analyst"},
		{"Omar Haddad transferred from sales lead to regional director for EMEA at Northstar after the merger.", "Omar Haddad", "regional director", "Northstar", "role", "sales lead"},
		{"At Vertex Labs, junior engineer Tom Bradley assists principal engineer Anna Cole on the platform team.", "Anna Cole", "principal engineer", "Vertex Labs", "name", "Tom Bradley"},
		{"Greenfield's CFO, Mia Lund, was previously CFO at rival firm Oakline.", "Mia Lund", "CFO", "Greenfield", "org", "Oakline"},
		{"Diego Ramos chairs the board at Solara, where his deputy Ines Roca runs day-to-day operations as COO.", "Ines Roca", "COO", "Solara", "name", "Diego Ramos"},
		{"Once the CTO of Bytewave, Karen Funk now leads Cloudgate as its CEO.", "Karen Funk", "CEO", "Cloudgate", "role", "CTO"},
	}
}

type extract3 struct{ Name, Role, Org string }

func (e extract3) get(field string) string {
	switch field {
	case "name":
		return e.Name
	case "role":
		return e.Role
	default:
		return e.Org
	}
}
func (e *extract3) set(field, v string) {
	switch field {
	case "name":
		e.Name = v
	case "role":
		e.Role = v
	default:
		e.Org = v
	}
}

type uncRow struct {
	idx              int
	tier             int // 0 faithful, 1 unrelated swap, 2 distractor swap
	ex               extract3
	schemaFaithful   bool
	groundedFaithful bool
	proseFaithful    bool
	proseParsed      bool
	fullFaithful     bool
	fullParsed       bool
	prose            string
}

func (c *UncoverHopCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	items := exItems()

	var rows []uncRow
	for i, it := range items {
		gold := extract3{it.Name, it.Role, it.Org}

		// Three instances per item: faithful, tier-1 (unrelated), tier-2 (distractor).
		faithful := gold

		tier1 := gold
		t1v := tier1Value(items, i)
		tier1.set(it.DField, t1v)

		tier2 := gold
		tier2.set(it.DField, it.DValue)

		for _, inst := range []struct {
			tier int
			ex   extract3
		}{{0, faithful}, {1, tier1}, {2, tier2}} {
			if inst.tier == 1 && t1v == "" {
				continue // no clean unrelated value available; skip rather than mislabel
			}
			rows = append(rows, c.scoreInstance(ctx, client, i, inst.tier, it.Text, inst.ex))
		}
	}

	uncReport(rows, c.Verbose)
	return nil
}

func (c *UncoverHopCmd) scoreInstance(ctx context.Context, client *llm.Client, idx, tier int, text string, ex extract3) uncRow {
	full, fullParsed := judgeExtractFull(ctx, client, text, ex)
	prose := proseExtractDiff(ctx, client, text, ex)
	proseFaithful, proseParsed := judgeExtractProse(ctx, client, text, prose)
	return uncRow{
		idx:              idx,
		tier:             tier,
		ex:               ex,
		schemaFaithful:   detSchema(ex),
		groundedFaithful: detGrounded(text, ex),
		proseFaithful:    proseFaithful,
		proseParsed:      proseParsed,
		fullFaithful:     full,
		fullParsed:       fullParsed,
		prose:            prose,
	}
}

// tier1Value picks an unrelated wrong value for item i's distractor field: the
// same field's gold value from another item that is NOT present in item i's
// text (so a substring-grounding check will catch it). Deterministic.
func tier1Value(items []exItem, i int) string {
	it := items[i]
	want := extract3{it.Name, it.Role, it.Org}.get(it.DField)
	lt := strings.ToLower(it.Text)
	for d := 1; d < len(items); d++ {
		o := items[(i+d)%len(items)]
		cand := extract3{o.Name, o.Role, o.Org}.get(it.DField)
		if strings.EqualFold(cand, want) {
			continue
		}
		if !strings.Contains(lt, strings.ToLower(cand)) {
			return cand
		}
	}
	return ""
}

// --- deterministic guardrails ---

func detSchema(e extract3) bool { // valid, non-empty → "faithful" (the weak check)
	return e.Name != "" && e.Role != "" && e.Org != ""
}

func detGrounded(text string, e extract3) bool { // every field must appear in source
	lt := strings.ToLower(text)
	for _, f := range []string{e.Name, e.Role, e.Org} {
		if !strings.Contains(lt, strings.ToLower(strings.TrimSpace(f))) {
			return false
		}
	}
	return true
}

// --- fuzzy channels ---

const exJudgeBar = `Reply with EXACTLY ONE WORD and nothing else: FAITHFUL if every extracted field (name, role, org) is the correct value for the person the source describes, or DRIFT if any field is wrong or names the wrong entity — even if that wrong value also appears somewhere in the source (e.g. a former employer or a different person mentioned).`

func judgeExtractFull(ctx context.Context, c *llm.Client, text string, e extract3) (bool, bool) {
	sys := "You verify a structured extraction against its source sentence. " + exJudgeBar
	p := fmt.Sprintf("SOURCE: %s\nEXTRACTION: name=%q role=%q org=%q", text, e.Name, e.Role, e.Org)
	r, err := c.Classify(ctx, llm.ModelOpus, sys, p, 16)
	if err != nil {
		return false, false
	}
	return parseVerdict(r.Text)
}

func proseExtractDiff(ctx context.Context, c *llm.Client, text string, e extract3) string {
	sys := `You compare a structured extraction against a source sentence. In 25 words or fewer, name any field (name/role/org) whose extracted value is wrong or refers to the wrong entity for the person described — including values that appear in the source but describe a different person, a former role, or a former employer. Say "all correct" if none. Report observable mismatches only; do NOT output a verdict.`
	p := fmt.Sprintf("SOURCE: %s\nEXTRACTION: name=%q role=%q org=%q", text, e.Name, e.Role, e.Org)
	r, err := c.Complete(ctx, llm.ModelHaiku, sys, p, 60)
	if err != nil {
		return "(summary unavailable)"
	}
	return strings.TrimSpace(r.Text)
}

func judgeExtractProse(ctx context.Context, c *llm.Client, text, prose string) (bool, bool) {
	sys := "A reviewer compared an extraction against a source sentence and described any field mismatches below. Based ONLY on this report, " + exJudgeBar
	r, err := c.Classify(ctx, llm.ModelOpus, sys, "Mismatch report: "+prose, 16)
	if err != nil {
		return false, false
	}
	return parseVerdict(r.Text)
}

// --- report ---

func uncReport(rows []uncRow, verbose bool) {
	var schema, grounded, prose, full confusion
	// Residual = tier-2 drift (what det-grounded misses). Recall on it, per channel.
	var resFull, resProse confusion // confusion restricted to tier-2 rows
	parseFail, drift, tier1, tier2 := 0, 0, 0, 0

	for _, r := range rows {
		isDrift := r.tier != 0
		if isDrift {
			drift++
		}
		if r.tier == 1 {
			tier1++
		}
		if r.tier == 2 {
			tier2++
		}
		if !r.fullParsed || !r.proseParsed {
			parseFail++
		}
		schema.add(!r.schemaFaithful, isDrift)
		grounded.add(!r.groundedFaithful, isDrift)
		if r.proseParsed {
			prose.add(!r.proseFaithful, isDrift)
		}
		if r.fullParsed {
			full.add(!r.fullFaithful, isDrift)
		}
		if r.tier == 2 { // the uncovered residual
			if r.fullParsed {
				resFull.add(!r.fullFaithful, true)
			}
			if r.proseParsed {
				resProse.add(!r.proseFaithful, true)
			}
		}
	}
	n := len(rows)

	if parseFail > 0 {
		fmt.Printf("⚠ INSTRUMENT WARNING: %d/%d rows had an unparseable LLM verdict; treat as suspect if high.\n\n", parseFail, n)
	}

	if verbose {
		fmt.Println("=== per-item (tier | schema/grounded/prose/full faithful) ===")
		for _, r := range rows {
			tl := []string{"faithful", "tier1-unrelated", "tier2-distractor"}[r.tier]
			fmt.Printf("  %2d %-16s ex{%q,%q,%q} sch=%-5v grd=%-5v prose=%-5s full=%-5s | %q\n",
				r.idx, tl, r.ex.Name, r.ex.Role, r.ex.Org,
				r.schemaFaithful, r.groundedFaithful,
				verdictStr(r.proseFaithful, r.proseParsed), verdictStr(r.fullFaithful, r.fullParsed),
				truncate(r.prose, 54))
		}
		fmt.Println()
	}

	fmt.Printf("population: N=%d (%d faithful, %d drift = %d tier-1 + %d tier-2)\n\n", n, n-drift, drift, tier1, tier2)

	fmt.Println("=== deterministic guardrail coverage g (no API) — the checkable fraction ===")
	fmt.Printf("  det-schema   (valid non-empty JSON):     g = %d/%d = %.2f   acc=%.2f prec=%.2f\n",
		schema.tp, drift, schema.recall(), schema.accuracy(), schema.precision())
	fmt.Printf("  det-grounded (every field in source):    g = %d/%d = %.2f   acc=%.2f prec=%.2f\n",
		grounded.tp, drift, grounded.recall(), grounded.accuracy(), grounded.precision())
	fmt.Printf("  → tier-2 (distractor-in-source) is the UNCOVERED residual: %d drift a substring check cannot catch.\n\n", tier2)

	fmt.Println("=== fuzzy channel recovery of the uncovered residual (tier-2 only, vs hard labels) ===")
	fmt.Printf("  R_full  (supervisor, full source)  = %d/%d = %.2f\n", resFull.tp, tier2, resFull.recall())
	fmt.Printf("  R_prose (supervisor, prose diff)   = %d/%d = %.2f\n", resProse.tp, tier2, resProse.recall())
	fmt.Printf("  per-hop loss λ = R_full − R_prose   = %+.2f   (grounded against labels, not a model verdict)\n\n",
		resFull.recall()-resProse.recall())

	fmt.Println("=== each channel's overall accuracy vs hard labels ===")
	fmt.Printf("  %-12s acc=%.2f prec=%.2f recall=%.2f\n", "det-schema", schema.accuracy(), schema.precision(), schema.recall())
	fmt.Printf("  %-12s acc=%.2f prec=%.2f recall=%.2f\n", "det-grounded", grounded.accuracy(), grounded.precision(), grounded.recall())
	fmt.Printf("  %-12s acc=%.2f prec=%.2f recall=%.2f\n", "prose", prose.accuracy(), prose.precision(), prose.recall())
	fmt.Printf("  %-12s acc=%.2f prec=%.2f recall=%.2f  (reference judge — a model, not an oracle)\n",
		"full", full.accuracy(), full.precision(), full.recall())

	fmt.Println("\nThe live question is R_prose vs R_full on tier-2: does compressing the supervisor's")
	fmt.Println("signal to a prose diff lose catching-power on the residual a deterministic check misses?")
	fmt.Println("Verify against --verbose per-item rows before recording any number as a finding.")
}
