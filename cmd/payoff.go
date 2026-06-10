package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// PayoffCmd measures the actual value proposition nothing else has: does
// shifting a mechanical chore from the frontier (Opus) to a cheaper tier
// (Haiku) BEHIND A GATE buy latency at held quality? It is the first
// experiment about the payoff rather than the safety discipline.
//
// Chore: extract {name, role, org} from a sentence (known gold). Three views:
//   - Opus baseline (the frontier cost we're trying to avoid)
//   - Haiku raw (the cheap tier — faster, worse)
//   - Haiku behind a DETERMINISTIC gate (substring-grounding, no API): serve
//     Haiku when the gate accepts, escalate to Opus when it rejects.
//
// The deterministic gate is chosen deliberately: an LLM verifier would hold
// quality but reintroduce an Opus round-trip per item, erasing the latency
// win (the breakeven the panel flagged). A deterministic gate preserves the
// win — at the cost of its g<1 residual, which this command measures as
// LEAKED errors (Haiku wrong, gate accepted, served wrong). That honest cost
// is the point, not a footnote.
//
// Latency is real wall-clock per live API call (llm.Result.LatencyMS),
// persisted in cache so reruns report the originally-measured times. Both
// tiers use thinking-disabled completion so the comparison is a clean
// mechanical-chore size difference, not a thinking-budget confound. Caveat:
// single sample per item — latencies are indicative, not a benchmark.
type PayoffCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool   `help:"Dump per-item tier outputs, gate decision, and latencies."`
}

type payoffRow struct {
	idx                                      int
	opusCorrect, sonnetCorrect, haikuCorrect bool
	gateAccept                               bool
	servedCorrect                            bool
	leaked                                   bool // gate accepted a wrong Haiku answer → served wrong
	opusLat, sonnetLat, haikuLat             int64
	gold, opusEx, sonnetEx, haikuEx          string // for verifying real-error vs exact-match-gold artifact
}

func (c *PayoffCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	items := exItems()

	var rows []payoffRow
	for i, it := range items {
		opusEx, opusLat := extractTimed(ctx, client, llm.ModelOpus, it.Text)
		sonnetEx, sonnetLat := extractTimed(ctx, client, llm.ModelSonnet, it.Text)
		haikuEx, haikuLat := extractTimed(ctx, client, llm.ModelHaiku, it.Text)

		opusCorrect := goldMatch(opusEx, it)
		sonnetCorrect := goldMatch(sonnetEx, it)
		haikuCorrect := goldMatch(haikuEx, it)
		hk := extract3{haikuEx.Name, haikuEx.Role, haikuEx.Org}
		// Deterministic gate = schema (non-empty) AND substring-grounding. The
		// non-empty leg is load-bearing: substring-grounding alone accepts ""
		// (Contains(x,"")==true), which would silently pass an empty extraction.
		gateAccept := detSchema(hk) && detGrounded(it.Text, hk)

		// Gated cascade: serve Haiku if the gate accepts, else escalate to Opus.
		servedCorrect := opusCorrect
		if gateAccept {
			servedCorrect = haikuCorrect
		}
		rows = append(rows, payoffRow{
			idx: i, opusCorrect: opusCorrect, sonnetCorrect: sonnetCorrect, haikuCorrect: haikuCorrect,
			gateAccept: gateAccept, servedCorrect: servedCorrect,
			leaked:  gateAccept && !haikuCorrect,
			opusLat: opusLat, sonnetLat: sonnetLat, haikuLat: haikuLat,
			gold:     fmt.Sprintf("%q/%q/%q", it.Name, it.Role, it.Org),
			opusEx:   fmt.Sprintf("%q/%q/%q", opusEx.Name, opusEx.Role, opusEx.Org),
			sonnetEx: fmt.Sprintf("%q/%q/%q", sonnetEx.Name, sonnetEx.Role, sonnetEx.Org),
			haikuEx:  fmt.Sprintf("%q/%q/%q", haikuEx.Name, haikuEx.Role, haikuEx.Org),
		})
	}

	payoffReport(rows, c.Verbose)
	return nil
}

func extractTimed(ctx context.Context, c *llm.Client, model, text string) (extraction, int64) {
	r, err := c.Classify(ctx, model, extractSys, text, 120) // thinking disabled → clean latency
	if err != nil {
		return extraction{}, 0
	}
	ex, _ := parseExtraction(r.Text)
	return ex, r.LatencyMS
}

// goldMatch uses bidirectional containment, not exact equality, so a
// more-complete-but-correct answer ("regional director for EMEA" vs gold
// "regional director") is NOT scored as an error — the exact-match-gold
// artifact that invalidated the earlier `experiment` run and recurred here.
// Empty never matches. Heuristic, not an LLM judge (kept out of the headline
// quality number deliberately); a wrong-but-disjoint answer still fails.
func goldMatch(ex extraction, it exItem) bool {
	return fieldEq(ex.Name, it.Name) && fieldEq(ex.Role, it.Role) && fieldEq(ex.Org, it.Org)
}

func fieldEq(got, gold string) bool {
	g, w := norm(got), norm(gold)
	if g == "" {
		return false
	}
	return g == w || strings.Contains(g, w) || strings.Contains(w, g)
}

func payoffReport(rows []payoffRow, verbose bool) {
	n := len(rows)
	opusAcc, sonnetAcc, haikuAcc, served, accepted, escalated, leaked := 0, 0, 0, 0, 0, 0, 0
	var opusLats, sonnetLats, haikuLats, gatedLats []int64
	for _, r := range rows {
		if r.opusCorrect {
			opusAcc++
		}
		if r.sonnetCorrect {
			sonnetAcc++
		}
		if r.haikuCorrect {
			haikuAcc++
		}
		if r.servedCorrect {
			served++
		}
		if r.gateAccept {
			accepted++
		} else {
			escalated++
		}
		if r.leaked {
			leaked++
		}
		opusLats = append(opusLats, r.opusLat)
		sonnetLats = append(sonnetLats, r.sonnetLat)
		haikuLats = append(haikuLats, r.haikuLat)
		// gated cost: Haiku for accepted; Haiku + Opus for escalated.
		g := r.haikuLat
		if !r.gateAccept {
			g += r.opusLat
		}
		gatedLats = append(gatedLats, g)
	}

	if verbose {
		fmt.Println("=== per-item (gate accept? served correct? latencies ms) ===")
		for _, r := range rows {
			tag := "escalate→opus"
			if r.gateAccept {
				tag = "serve haiku  "
			}
			leak := ""
			if r.leaked {
				leak = "  ⚠ LEAKED (served wrong)"
			}
			fmt.Printf("  %2d opus=%-5v sonnet=%-5v haiku=%-5v gate=%s served=%-5v  lat o/s/h=%d/%d/%d%s\n",
				r.idx, r.opusCorrect, r.sonnetCorrect, r.haikuCorrect, tag, r.servedCorrect, r.opusLat, r.sonnetLat, r.haikuLat, leak)
			if !r.opusCorrect || !r.sonnetCorrect || !r.haikuCorrect {
				fmt.Printf("       gold  =%s\n       opus  =%s\n       sonnet=%s\n       haiku =%s\n", r.gold, r.opusEx, r.sonnetEx, r.haikuEx)
			}
		}
		fmt.Println()
	}

	fmt.Printf("population: N=%d  (mechanical chore: extract {name,role,org})\n\n", n)

	fmt.Println("=== the cost gradient (shift-left is not binary — pick the cheapest tier that holds) ===")
	op := float64(max64(median(opusLats), 1))
	fmt.Printf("  opus    accuracy %d/%d = %.2f   median latency %5d ms   (100%% of opus)\n", opusAcc, n, frac(opusAcc, n), median(opusLats))
	fmt.Printf("  sonnet  accuracy %d/%d = %.2f   median latency %5d ms   (%.0f%% of opus)\n",
		sonnetAcc, n, frac(sonnetAcc, n), median(sonnetLats), 100*float64(median(sonnetLats))/op)
	fmt.Printf("  haiku   accuracy %d/%d = %.2f   median latency %5d ms   (%.0f%% of opus)\n",
		haikuAcc, n, frac(haikuAcc, n), median(haikuLats), 100*float64(median(haikuLats))/op)
	fmt.Printf("  → cheapest tier holding opus-quality on this chore: %s\n", cheapestHolding(opusAcc, sonnetAcc, haikuAcc))

	fmt.Println("\n=== haiku behind a DETERMINISTIC gate (substring-grounding, no API) ===")
	fmt.Printf("  gate accepted %d/%d, escalated to opus %d/%d\n", accepted, n, escalated, n)
	fmt.Printf("  served accuracy (haiku if accepted else opus) = %d/%d = %.2f\n", served, n, frac(served, n))
	fmt.Printf("  LEAKED (gate accepted a WRONG haiku answer → served wrong) = %d   ← the held-quality cost of g<1\n", leaked)

	fmt.Println("\n=== payoff: gated cascade vs always-opus ===")
	fmt.Printf("  accuracy:  gated %.2f   vs  always-opus %.2f   (delta %+.2f)\n", frac(served, n), frac(opusAcc, n), frac(served, n)-frac(opusAcc, n))
	fmt.Printf("  latency :  gated median %d ms   vs  always-opus median %d ms\n", median(gatedLats), median(opusLats))
	saved := 100 * (1 - float64(median(gatedLats))/float64(max64(median(opusLats), 1)))
	fmt.Printf("  latency saved ≈ %.0f%%   (bought at the cost of %d leaked error(s) the deterministic gate can't see)\n", saved, leaked)

	fmt.Println("\nTwo knobs, not one: (a) WHICH TIER (gradient above — you needn't shift all the way to")
	fmt.Println("haiku if sonnet holds quality at lower latency), and (b) the GATE — deterministic keeps the")
	fmt.Println("latency win but leaks its g<1 residual; an LLM verifier holds quality but re-adds an opus")
	fmt.Println("round-trip, erasing it. Local models are the gradient's far end (unmeasured). Verify")
	fmt.Println("per-item --verbose (esp. leaked rows) before trusting any of this.")
}

func median(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func frac(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// cheapestHolding names the cheapest tier whose accuracy still matches Opus on
// this chore — the brief's actual selection rule ("cheapest tier that
// realistically succeeds"), not a binary jump to the bottom.
func cheapestHolding(opus, sonnet, haiku int) string {
	if haiku >= opus {
		return "haiku"
	}
	if sonnet >= opus {
		return "sonnet (haiku drops quality)"
	}
	return "opus (both cheaper tiers drop quality here)"
}
