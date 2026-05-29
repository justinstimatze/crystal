package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// DepthSweepCmd tests the one claim neither hop reached: does content-bearing
// up-channel loss COMPOUND over depth? ground-hop and uncover-hop both measured
// per-hop loss λ≈0 at depth 1. The lattice's pessimism (safe depth ~2) rests on
// λ>0 per hop; STACKING #7's worry is that "a prose summary loses fidelity at
// every hop." This relays the prose channel up through k lossy paraphrase hops
// and measures whether the supervisor can still catch the uncovered semantic
// error at each depth.
//
// Substrate: uncover-hop's tier-2 residual (semantic distractor errors no
// substring check catches) plus the faithful items (to detect false alarms
// creeping in as the relayed signal degrades). At each hop a cheap tier (Haiku)
// rewrites the prior report in its own words under a tight budget — modelling a
// layer summarising the layer below. The supervisor (Opus) judges from the
// depth-d report alone.
//
//	depth 1 = supervisor reads the original concrete diff (== uncover-hop).
//	depth d = supervisor reads the diff after d−1 paraphrase relays.
//
// Result is the curve R_tier2(d): flat → content survives depth (the lattice's
// shallow-safe-depth pessimism is wrong for content-bearing channels); cliffs
// at some d → we have MEASURED the safe depth instead of assuming it.
type DepthSweepCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Depth    int    `help:"Max relay depth to sweep." default:"6"`
	Verbose  bool   `help:"Dump the relayed report chain + verdict at each depth (the artifact that shows whether content washes out)."`
}

type sweepRow struct {
	idx     int
	drift   bool // true = tier-2 distractor, false = faithful
	diffs   []string
	verdict []bool // faithful verdict at each depth
	parsed  []bool
}

func (c *DepthSweepCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	if c.Depth < 1 {
		return usageError{fmt.Errorf("depth must be >= 1")}
	}
	ctx := context.Background()
	items := exItems()

	var rows []sweepRow
	for i, it := range items {
		gold := extract3{it.Name, it.Role, it.Org}
		tier2 := gold
		tier2.set(it.DField, it.DValue)
		for _, inst := range []struct {
			drift bool
			ex    extract3
		}{{false, gold}, {true, tier2}} {
			rows = append(rows, c.sweepInstance(ctx, client, i, inst.drift, it.Text, inst.ex))
		}
	}

	sweepReport(rows, c.Depth, c.Verbose)
	return nil
}

func (c *DepthSweepCmd) sweepInstance(ctx context.Context, client *llm.Client, idx int, drift bool, text string, ex extract3) sweepRow {
	diffs := make([]string, c.Depth)
	diffs[0] = proseExtractDiff(ctx, client, text, ex) // depth-1 input (cache hit from uncover-hop)
	for h := 1; h < c.Depth; h++ {
		diffs[h] = relaySummary(ctx, client, diffs[h-1])
	}
	verdict := make([]bool, c.Depth)
	parsed := make([]bool, c.Depth)
	for d := 0; d < c.Depth; d++ {
		verdict[d], parsed[d] = judgeExtractProse(ctx, client, "", diffs[d])
	}
	return sweepRow{idx: idx, drift: drift, diffs: diffs, verdict: verdict, parsed: parsed}
}

// relaySummary models one upward hop: a cheap tier rewrites the report in its
// own words under a tight budget, preserving any specific field claim. Forcing
// paraphrase (not verbatim copy) is what gives compounding loss a real chance.
func relaySummary(ctx context.Context, c *llm.Client, prior string) string {
	sys := `You are relaying a status report up one level to your supervisor. Rewrite it in 15 words or fewer, IN YOUR OWN WORDS — do not copy it verbatim. Preserve any specific claim that a particular field (name, role, or org) is wrong, including which field and the correct value. If the report says everything is correct, say so plainly.`
	r, err := c.Complete(ctx, llm.ModelHaiku, sys, "Report from below: "+prior, 40)
	if err != nil {
		return "(relay unavailable)"
	}
	return strings.TrimSpace(r.Text)
}

func sweepReport(rows []sweepRow, depth int, verbose bool) {
	driftN, faithN := 0, 0
	for _, r := range rows {
		if r.drift {
			driftN++
		} else {
			faithN++
		}
	}

	if verbose {
		fmt.Println("=== relayed report chain per item (does the specific error survive?) ===")
		for _, r := range rows {
			tag := "faithful"
			if r.drift {
				tag = "DRIFT(tier2)"
			}
			fmt.Printf("item %d [%s]\n", r.idx, tag)
			for d := 0; d < depth; d++ {
				fmt.Printf("   d%d v=%-5s | %q\n", d+1, verdictStr(r.verdict[d], r.parsed[d]), truncate(r.diffs[d], 72))
			}
		}
		fmt.Println()
	}

	fmt.Printf("population: %d tier-2 (uncovered semantic drift) + %d faithful, swept to depth %d\n\n", driftN, faithN, depth)
	fmt.Println("depth | R_tier2 (recall on uncovered drift) | FP_faithful (false alarms) | parse-fail")
	fmt.Println("------+--------------------------------------+----------------------------+-----------")
	for d := 0; d < depth; d++ {
		caughtDrift, parsedDrift := 0, 0
		fpFaith, parsedFaith := 0, 0
		parseFail := 0
		for _, r := range rows {
			if !r.parsed[d] {
				parseFail++
				continue
			}
			if r.drift {
				parsedDrift++
				if !r.verdict[d] { // predicted DRIFT = caught
					caughtDrift++
				}
			} else {
				parsedFaith++
				if !r.verdict[d] { // predicted DRIFT on a faithful = false alarm
					fpFaith++
				}
			}
		}
		fmt.Printf("  %2d  | %2d/%-2d = %.2f%-23s| %2d/%-2d = %.2f%-15s| %d\n",
			d+1, caughtDrift, parsedDrift, safeRatio(caughtDrift, parsedDrift), "",
			fpFaith, parsedFaith, safeRatio(fpFaith, parsedFaith), "", parseFail)
	}

	fmt.Println("\nFlat R_tier2 across depth → content-bearing channel does NOT compound loss (the")
	fmt.Println("lattice's shallow-safe-depth pessimism would be unsupported for such channels).")
	fmt.Println("A cliff at depth d → the safe depth is MEASURED, not assumed. Rising FP_faithful →")
	fmt.Println("the relay degrades into noise. Read the --verbose chain to see WHY before trusting the curve.")
}
