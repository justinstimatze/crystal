package cmd

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/justinstimatze/crystal/internal/teamsolve"
)

// TeamRegradeCmd re-runs the hidden tests on every saved solution.py of a
// team-solve output dir, with bounded parallelism and nothing else running.
// Found 2026-11-03: grading inside a live run shares the CPU with every other
// problem in flight, so a correct-but-slow Python program flips between pass
// and timeout on the same code depending on load (same program, 43/43 in one
// run, 3/43 in another). Grading must be a separate, uncontended step.
type TeamRegradeCmd struct {
	Problems    string  `help:"The problems jsonl the run used." required:""`
	Out         string  `help:"A team-solve --out dir (reads its results.tsv and passN/<id>/<arm>/solution.py)." required:""`
	Concurrency int     `help:"Programs graded at once; keep it under the CPU count." default:"2"`
	Limit       float64 `help:"Seconds per hidden test." default:"6"`
	Python      string  `default:"python3"`
}

func (c *TeamRegradeCmd) Run() error {
	probs, _, err := teamsolve.Load(c.Problems)
	if err != nil {
		return usageError{err}
	}
	byID := map[string]teamsolve.Problem{}
	for _, p := range probs {
		byID[p.ID] = p
	}
	f, err := os.Open(filepath.Join(c.Out, "results.tsv"))
	if err != nil {
		return usageError{err}
	}
	rd := csv.NewReader(f)
	rd.Comma, rd.LazyQuotes, rd.FieldsPerRecord = '\t', true, -1
	recs, err := rd.ReadAll()
	f.Close()
	if err != nil {
		return err
	}
	head, rows := recs[0], recs[1:]
	col := map[string]int{}
	for i, h := range head {
		col[h] = i
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(1, c.Concurrency))
	var mu sync.Mutex
	flips := 0
	for i := range rows {
		r := rows[i]
		p, ok := byID[r[col["id"]]]
		prog := filepath.Join(c.Out, "pass"+r[col["pass"]], r[col["id"]], r[col["arm"]], "solution.py")
		code, err := os.ReadFile(prog)
		if !ok || err != nil || r[col["err"]] != "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			all := append(append([]teamsolve.Case{}, p.Public...), p.Hidden...)
			s := teamsolve.RunSuite(context.Background(), c.Python, string(code), all, time.Duration(c.Limit*float64(time.Second)), true)
			solved := fmt.Sprint(s.All())
			mu.Lock()
			if solved != r[col["solved"]] {
				flips++
				fmt.Fprintf(os.Stderr, "  flip %s p%s %s: %s -> %s (%d/%d)\n", r[col["id"]], r[col["pass"]], r[col["arm"]], r[col["solved"]], solved, s.Passed, s.Total)
			}
			r[col["solved"]] = solved
			r[col["hidden"]] = fmt.Sprintf("%d/%d", s.Passed, s.Total)
			mu.Unlock()
		}()
	}
	wg.Wait()
	out := filepath.Join(c.Out, "results-regraded.tsv")
	var b strings.Builder
	for _, r := range append([][]string{head}, rows...) {
		b.WriteString(strings.Join(r, "\t") + "\n")
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("regraded %d rows, %d verdicts changed -> %s\n", len(rows), flips, out)
	return nil
}
