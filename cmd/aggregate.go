package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/justinstimatze/crystal/internal/llm"
)

// AggregateCmd hunts a cheap-model limit and tests where decomposition finally
// pays: semantic aggregation — "how many of these N items match a semantic
// criterion?" A string tool can't classify semantically; a whole-task cheap
// model has to classify AND count over many items in one pass (a known weak
// spot). The decomposition is map-reduce: cheap model classifies each item in
// isolation (a focused sub-task), deterministic code counts the yeses.
//
// Conditions per count-task:
//   - opus-whole   : Opus counts over the whole list (frontier baseline)
//   - haiku-whole  : Haiku counts over the whole list (expect aggregate errors)
//   - haiku-mapred : Haiku classifies each item YES/NO; Go counts (decomposition)
//
// Hard labels: each item is annotated, so the true count is exact.
type AggregateCmd struct {
	CacheDir string `help:"Disk cache dir for LLM calls." default:".crystal-cache"`
	Verbose  bool   `help:"Dump per-task true/opus/haiku/mapred counts and per-item map verdicts."`
}

type aggItem struct {
	text string
	cats map[string]bool
}
type aggQuestion struct{ list, cat, prompt string }

func aggLists() map[string][]aggItem {
	return map[string][]aggItem{
		"reviews": {
			{"Arrived three days late and the box was crushed.", map[string]bool{"delivery": true, "price": false}},
			{"Great product but honestly overpriced for what it is.", map[string]bool{"delivery": false, "price": true}},
			{"Shipping took forever, I almost canceled my order.", map[string]bool{"delivery": true, "price": false}},
			{"Works perfectly, fast shipping, very happy.", map[string]bool{"delivery": false, "price": false}},
			{"Way too expensive, found the same thing cheaper elsewhere.", map[string]bool{"delivery": false, "price": true}},
			{"The courier left it at the wrong address and it took a week to sort out.", map[string]bool{"delivery": true, "price": false}},
			{"Decent quality, fair price, no complaints.", map[string]bool{"delivery": false, "price": false}},
			{"Item was fine but delivery was a nightmare and it cost too much to ship.", map[string]bool{"delivery": true, "price": true}},
		},
		"bugs": {
			{"App crashes when I rotate the screen on the profile page.", map[string]bool{"security": false, "crash": true}},
			{"User passwords are stored in plaintext in the logs.", map[string]bool{"security": true, "crash": false}},
			{"The export button does nothing on Firefox.", map[string]bool{"security": false, "crash": false}},
			{"Anyone can access another user's invoices by changing the URL id.", map[string]bool{"security": true, "crash": false}},
			{"Segfault on startup if the config file is missing.", map[string]bool{"security": false, "crash": true}},
			{"Session tokens never expire, even after logout.", map[string]bool{"security": true, "crash": false}},
			{"Typo on the settings page header.", map[string]bool{"security": false, "crash": false}},
			{"The app freezes and then closes when uploading a large file.", map[string]bool{"security": false, "crash": true}},
		},
		"people": {
			{"Maya Chen is the chief executive of Drift Analytics.", map[string]bool{"ceo": true, "founder": false}},
			{"Before retiring, Tom Reyes led Orbit as CEO for a decade.", map[string]bool{"ceo": false, "founder": false}},
			{"Aisha Okoro founded and still runs Verde as its CEO.", map[string]bool{"ceo": true, "founder": true}},
			{"Lena Park, who founded the studio, sold it years ago.", map[string]bool{"ceo": false, "founder": true}},
			{"Sam Idris was named CEO of Halcyon last month.", map[string]bool{"ceo": true, "founder": false}},
			{"Dev Rao stepped down as CEO of Nimbus in March.", map[string]bool{"ceo": false, "founder": false}},
			{"Priya Nair co-founded the company but never held an executive title.", map[string]bool{"ceo": false, "founder": true}},
			{"Omar Vance currently serves as chief executive of Brightline.", map[string]bool{"ceo": true, "founder": false}},
		},
	}
}

func aggQuestions() []aggQuestion {
	return []aggQuestion{
		{"reviews", "delivery", "the reviewer complains about slow, late, or failed delivery/shipping"},
		{"reviews", "price", "the reviewer complains that the price or cost is too high"},
		{"bugs", "security", "the report describes a security vulnerability (auth, access control, secrets, tokens)"},
		{"bugs", "crash", "the report describes the app crashing, freezing, or closing unexpectedly"},
		{"people", "ceo", "the person CURRENTLY serves as CEO / chief executive (not a former or stepped-down one)"},
		{"people", "founder", "the person founded or co-founded the company/studio"},
	}
}

type aggRow struct {
	q                            aggQuestion
	truth, opus, haiku, mapred   int
	opusOK, haikuOK              bool // count parsed
	opusLat, haikuLat, mapredLat int64
	mapVerdicts                  []bool
}

func (c *AggregateCmd) Run() error {
	client, err := llm.New(c.CacheDir)
	if err != nil {
		return usageError{err}
	}
	ctx := context.Background()
	lists := aggLists()

	var rows []aggRow
	for _, q := range aggQuestions() {
		items := lists[q.list]
		truth := 0
		for _, it := range items {
			if it.cats[q.cat] {
				truth++
			}
		}
		opus, opusOK, opusLat := wholeCount(ctx, client, llm.ModelOpus, items, q.prompt)
		haiku, haikuOK, haikuLat := wholeCount(ctx, client, llm.ModelHaiku, items, q.prompt)
		mapred, mverds, mlat := mapReduceCount(ctx, client, items, q.prompt)
		rows = append(rows, aggRow{
			q: q, truth: truth, opus: opus, haiku: haiku, mapred: mapred,
			opusOK: opusOK, haikuOK: haikuOK,
			opusLat: opusLat, haikuLat: haikuLat, mapredLat: mlat, mapVerdicts: mverds,
		})
	}
	aggReport(rows, c.Verbose)
	return nil
}

func wholeCount(ctx context.Context, c *llm.Client, model string, items []aggItem, prompt string) (int, bool, int64) {
	var b strings.Builder
	for i, it := range items {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it.text)
	}
	sys := fmt.Sprintf("Count how many of the numbered items match this criterion: %s. Reply with ONLY the number.", prompt)
	r, err := c.Classify(ctx, model, sys, b.String(), 12)
	if err != nil {
		return -1, false, 0
	}
	n, ok := firstInt(r.Text)
	return n, ok, r.LatencyMS
}

func mapReduceCount(ctx context.Context, c *llm.Client, items []aggItem, prompt string) (int, []bool, int64) {
	sys := fmt.Sprintf("Does this single item match the criterion: %s? Reply YES or NO only.", prompt)
	count := 0
	var verds []bool
	var lat int64
	for _, it := range items {
		r, err := c.Classify(ctx, llm.ModelHaiku, sys, it.text, 6)
		yes := err == nil && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(r.Text)), "YES")
		if yes {
			count++
		}
		verds = append(verds, yes)
		lat += r.LatencyMS // sequential cost; parallelizable to ~one call
	}
	return count, verds, lat
}

func firstInt(s string) (int, bool) {
	cur := ""
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			cur += string(ch)
		} else if cur != "" {
			break
		}
	}
	if cur == "" {
		return -1, false
	}
	n, err := strconv.Atoi(cur)
	return n, err == nil
}

func aggReport(rows []aggRow, verbose bool) {
	n := len(rows)
	opusExact, haikuExact, mapExact := 0, 0, 0
	opusAbs, haikuAbs, mapAbs := 0, 0, 0
	var opusLats, haikuLats, mapLats []int64
	opusPF, haikuPF := 0, 0
	for _, r := range rows {
		if r.opusOK {
			if r.opus == r.truth {
				opusExact++
			}
			opusAbs += abs(r.opus - r.truth)
		} else {
			opusPF++
		}
		if r.haikuOK {
			if r.haiku == r.truth {
				haikuExact++
			}
			haikuAbs += abs(r.haiku - r.truth)
		} else {
			haikuPF++
		}
		if r.mapred == r.truth {
			mapExact++
		}
		mapAbs += abs(r.mapred - r.truth)
		opusLats = append(opusLats, r.opusLat)
		haikuLats = append(haikuLats, r.haikuLat)
		mapLats = append(mapLats, r.mapredLat)
	}

	if verbose {
		fmt.Println("=== per count-task (truth | opus / haiku-whole / haiku-mapreduce) ===")
		for _, r := range rows {
			fmt.Printf("  %-8s %-9s truth=%d  opus=%d haiku-whole=%d mapred=%d  (map: %s)\n",
				r.q.list, r.q.cat, r.truth, r.opus, r.haiku, r.mapred, boolsToBits(r.mapVerdicts))
		}
		fmt.Println()
	}

	fmt.Printf("population: %d count-tasks (8 items each) · semantic aggregation\n\n", n)
	fmt.Println("=== exact-count accuracy + mean |error| + latency ===")
	fmt.Printf("  opus-whole       exact %d/%d   mean|err| %.2f   median %d ms   parse-fail %d\n",
		opusExact, n, float64(opusAbs)/float64(n), median(opusLats), opusPF)
	fmt.Printf("  haiku-whole      exact %d/%d   mean|err| %.2f   median %d ms   parse-fail %d\n",
		haikuExact, n, float64(haikuAbs)/float64(n), median(haikuLats), haikuPF)
	fmt.Printf("  haiku-mapreduce  exact %d/%d   mean|err| %.2f   median %d ms (8 sequential calls; parallelizable to ~1)\n",
		mapExact, n, float64(mapAbs)/float64(n), median(mapLats))

	fmt.Println("\nThesis: whole-task cheap model should miscount over many items; map-reduce (cheap")
	fmt.Println("per-item classify + deterministic count) should beat it and approach opus. If haiku-whole")
	fmt.Println("errs while mapred is exact, decomposition finally pays. Verify --verbose per-item first.")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func boolsToBits(bs []bool) string {
	var b strings.Builder
	for _, v := range bs {
		if v {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}
