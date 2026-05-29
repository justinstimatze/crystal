package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// ContentSweepCmd is the rigorous loop-closer for the depth sweep. depth-sweep
// found DETECTION recall flat at 1.00 across depth, but its verbose chains
// showed the corrective CONTENT (which field, the correct value) eroding and
// even inverting — a failure the binary metric could not see. This command
// measures that content fidelity directly: from each depth-d relayed report
// (blind to ground truth), a supervisor recovers the proposed correction
// {field, value}; that recovery is scored against the hard labels.
//
// Three outcomes per recovered value, two of them deterministic:
//
//	gold     — names the correct value (substring match, or a narrow equivalence
//	           judge for synonyms like CEO ≡ chief executive officer).
//	inverted — names the WRONG (corrupted) value as the fix — the clearest
//	           content failure; fully deterministic against the known distractor.
//	other    — flags a problem but recovers neither value.
//
// content-fidelity(d) = field-correct AND value=gold. The curve vs depth is the
// measured erosion depth-sweep could only show by eye.
type ContentSweepCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Depth    int    `help:"Max relay depth to sweep." default:"6"`
	Verbose  bool   `help:"Dump per-item recovered {field,value} vs ground truth at each depth."`
}

type cfRow struct {
	idx        int
	field      string // ground-truth corrupted field
	gold, dval string // ground-truth correct value / injected wrong value
	fieldOK    []bool
	valueClass []string // "gold" | "inverted" | "other" per depth
	parsed     []bool
	recovered  []string // recovered "field=.. value=.." for verbose
}

func (c *ContentSweepCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	if c.Depth < 1 {
		return usageError{fmt.Errorf("depth must be >= 1")}
	}
	ctx := context.Background()

	var rows []cfRow
	for i, it := range exItems() {
		gold := extract3{it.Name, it.Role, it.Org}
		goldVal := gold.get(it.DField)
		tier2 := gold
		tier2.set(it.DField, it.DValue)

		diffs := relayChain(ctx, client, it.Text, tier2, c.Depth) // cache hits

		row := cfRow{idx: i, field: it.DField, gold: goldVal, dval: it.DValue}
		for d := 0; d < c.Depth; d++ {
			f, v, ok := recoverCorrection(ctx, client, diffs[d])
			row.parsed = append(row.parsed, ok)
			row.fieldOK = append(row.fieldOK, ok && normalizeField(f) == it.DField)
			row.valueClass = append(row.valueClass, classifyValue(ctx, client, it.DField, v, goldVal, it.DValue, ok))
			row.recovered = append(row.recovered, fmt.Sprintf("field=%s value=%q", normalizeField(f), v))
		}
		rows = append(rows, row)
	}

	cfReport(rows, c.Depth, c.Verbose)
	return nil
}

// recoverCorrection asks the supervisor, BLIND to ground truth, what the report
// says the correction is. JSON out; thinking disabled so the budget isn't
// starved (the ground-hop lesson).
func recoverCorrection(ctx context.Context, c *llm.Client, report string) (field, value string, parsed bool) {
	sys := `A report below describes a problem with an extracted record's field. Based ONLY on the report, output the correction as JSON: {"field":"name|role|org","correct_value":"<the value that field SHOULD be>"}. If the report does not say, put your best reading. Output only JSON, nothing else.`
	r, err := c.Classify(ctx, llm.ModelOpus, sys, "Report: "+report, 40)
	if err != nil {
		return "", "", false
	}
	a, b := strings.Index(r.Text, "{"), strings.LastIndex(r.Text, "}")
	if a < 0 || b <= a {
		return "", "", false
	}
	var out struct {
		Field string `json:"field"`
		Value string `json:"correct_value"`
	}
	if json.Unmarshal([]byte(r.Text[a:b+1]), &out) != nil {
		return "", "", false
	}
	return out.Field, out.Value, true
}

// classifyValue scores a recovered value against the hard labels. Deterministic
// first (substring match to gold, then to the known distractor = inversion);
// only the ambiguous remainder hits a narrow equivalence judge (for synonyms).
func classifyValue(ctx context.Context, c *llm.Client, field, recovered, gold, dval string, parsed bool) string {
	if !parsed || strings.TrimSpace(recovered) == "" {
		return "other"
	}
	nr, ng, nd := norm(recovered), norm(gold), norm(dval)
	if nr == ng || strings.Contains(nr, ng) || strings.Contains(ng, nr) {
		return "gold"
	}
	if nr == nd || strings.Contains(nr, nd) || strings.Contains(nd, nr) {
		return "inverted"
	}
	if equivalentValue(ctx, c, field, recovered, gold) {
		return "gold"
	}
	return "other"
}

func equivalentValue(ctx context.Context, c *llm.Client, field, a, b string) bool {
	sys := fmt.Sprintf("For the %q field of a person record, do these two values refer to the same thing? Reply only YES or NO.", field)
	r, err := c.Classify(ctx, llm.ModelOpus, sys, fmt.Sprintf("A: %s\nB: %s", a, b), 8)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(r.Text)), "YES")
}

func normalizeField(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "org"), strings.Contains(l, "compan"), strings.Contains(l, "employer"):
		return "org"
	case strings.Contains(l, "role"), strings.Contains(l, "title"), strings.Contains(l, "position"):
		return "role"
	case strings.Contains(l, "name"):
		return "name"
	}
	return l
}

func cfReport(rows []cfRow, depth int, verbose bool) {
	if verbose {
		fmt.Println("=== recovered correction vs ground truth, per depth ===")
		for _, r := range rows {
			fmt.Printf("item %d  truth: %s should be %q (wrong=%q)\n", r.idx, r.field, r.gold, r.dval)
			for d := 0; d < depth; d++ {
				fok := "field✗"
				if r.fieldOK[d] {
					fok = "field✓"
				}
				fmt.Printf("   d%d %-7s value=%-9s | %s\n", d+1, fok, r.valueClass[d], r.recovered[d])
			}
		}
		fmt.Println()
	}

	n := len(rows)
	fmt.Printf("population: %d tier-2 items, depth %d\n\n", n, depth)
	fmt.Println("depth | field-acc | value=gold | inverted | other | content-fidelity (field✓ & gold) | parse-fail")
	fmt.Println("------+-----------+------------+----------+-------+----------------------------------+-----------")
	for d := 0; d < depth; d++ {
		fieldOK, gold, inv, other, content, pf := 0, 0, 0, 0, 0, 0
		for _, r := range rows {
			if !r.parsed[d] {
				pf++
				continue
			}
			if r.fieldOK[d] {
				fieldOK++
			}
			switch r.valueClass[d] {
			case "gold":
				gold++
			case "inverted":
				inv++
			default:
				other++
			}
			if r.fieldOK[d] && r.valueClass[d] == "gold" {
				content++
			}
		}
		fmt.Printf("  %2d  |   %2d/%-2d   |   %2d/%-2d    |   %2d     |  %2d   |            %2d/%-2d = %.2f            | %d\n",
			d+1, fieldOK, n, gold, n, inv, other, content, n, safeRatio(content, n), pf)
	}

	fmt.Println("\nDeclining content-fidelity with depth = corrective content compounds-loses even though")
	fmt.Println("detection stays flat (depth-sweep). Rising 'inverted' = the relay actively points the")
	fmt.Println("supervisor at the WRONG value. Verify the --verbose recoveries before trusting the curve.")
}
