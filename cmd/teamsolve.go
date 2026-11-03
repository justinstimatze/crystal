package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/justinstimatze/crystal/internal/publicai"
	"github.com/justinstimatze/crystal/internal/teamsolve"
)

// TeamSolveCmd runs memdoor's port of the GVS5H team method against a one-call
// baseline on LiveCodeBench stdin problems, on any OpenAI-compatible endpoint
// (the Hugging Face router by default, which serves Apertus 1.5 through Public
// AI). The loop sees only the samples; the score is every hidden test.
type TeamSolveCmd struct {
	Problems     string   `help:"jsonl from bench/teamsolve/lcb_filter.py." required:""`
	Model        string   `help:"Model for every role unless --role-model overrides." default:"swiss-ai/Apertus-v1.5-70B"`
	RoleModel    []string `help:"role=model override (roles: single, plan, ideate-a, ideate-b, solve, review, repair). Repeatable." name:"role-model"`
	Arms         []string `help:"Arms to run." default:"single,team,team-guard" enum:"single,team,team-guard,team-stress"`
	Only         []string `help:"Problem ids to run (default all)."`
	Limit        int      `help:"First N problems after --only (0 = all)." default:"0"`
	Passes       int      `help:"Independent passes per problem (each its own sample index, so no cache reuse)." default:"1"`
	FirstPass    int      `help:"Sample index of the first pass (run pass 2 alone with --first-pass 2)." default:"1" name:"first-pass"`
	Concurrency  int      `help:"Problems in flight." default:"4"`
	MaxTokens    int64    `help:"Reply cap per call." default:"8192"`
	TopP         float64  `help:"top_p (0 = provider default)." default:"0"`
	StressRounds int      `help:"team-stress: repair rounds driven by samples then the brute-force diff." default:"2" name:"stress-rounds"`
	NoThink      bool     `help:"Send chat_template_kwargs.enable_thinking=false (Qwen3.x hybrid models: thinking off, as memdoor's bench ran)." name:"no-think"`
	BaseURL      string   `help:"OpenAI-compatible base URL." default:"https://router.huggingface.co/v1"`
	KeyEnv       string   `help:"Env var holding the bearer key." default:"HF_TOKEN"`
	Price        []string `help:"model=IN/OUT dollars per 1M tokens, for the cost column. Repeatable." default:"swiss-ai/Apertus-v1.5-70B=0.82/2.92,Qwen/Qwen3.8-27B=0.20/2.50,swiss-ai/Apertus-8B-Instruct-2509=0.10/0.20"`
	Python       string   `help:"Interpreter for the programs." default:"python3"`
	Out          string   `help:"Output dir: results.tsv plus every prompt, reply and program." default:"teamsolve-out"`
	CacheDir     string   `help:"Disk cache dir." default:".crystal-cache"`
}

func (c *TeamSolveCmd) Run() error {
	key := os.Getenv(c.KeyEnv)
	if key == "" {
		return usageError{fmt.Errorf("%s not set", c.KeyEnv)}
	}
	client, err := publicai.NewAt(c.CacheDir, c.BaseURL, key)
	if err != nil {
		return usageError{err}
	}
	probs, skipped, err := teamsolve.Load(c.Problems)
	if err != nil {
		return usageError{err}
	}
	if len(c.Only) > 0 {
		want := map[string]bool{}
		for _, id := range c.Only {
			want[id] = true
		}
		var keep []teamsolve.Problem
		for _, p := range probs {
			if want[p.ID] {
				keep = append(keep, p)
			}
		}
		probs = keep
	}
	if c.Limit > 0 && c.Limit < len(probs) {
		probs = probs[:c.Limit]
	}
	models := map[string]string{"": c.Model}
	for _, rm := range c.RoleModel {
		r, m, ok := strings.Cut(rm, "=")
		if !ok {
			return usageError{fmt.Errorf("--role-model %q: want role=model", rm)}
		}
		models[r] = m
	}
	prices, err := parsePrices(c.Price)
	if err != nil {
		return usageError{err}
	}
	fmt.Fprintf(os.Stderr, "team-solve: %d stdin problems (%d functional skipped) × %d pass(es) × arms %v @ %s\n",
		len(probs), skipped, c.Passes, c.Arms, c.BaseURL)

	type job struct {
		p    teamsolve.Problem
		pass int
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var results []teamRowT
	if err := os.MkdirAll(c.Out, 0o755); err != nil {
		return err
	}
	tsv, err := os.Create(filepath.Join(c.Out, "results.tsv"))
	if err != nil {
		return err
	}
	defer tsv.Close()
	fmt.Fprintln(tsv, "id\tpass\tarm\tsolved\tpublic\thidden\tkept\tcalls\tin_tok\tout_tok\tcutoffs\tcost_usd\tseconds\terr\tstress")

	ctx := context.Background()
	var wg sync.WaitGroup
	for w := 0; w < max(1, c.Concurrency); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				cfg := teamsolve.Config{Models: models, MaxTokens: c.MaxTokens, TopP: c.TopP, NoThink: c.NoThink, Python: c.Python,
					Sample: j.pass, PublicLimit: 10 * time.Second, HiddenLimit: 6 * time.Second,
					TranscriptTo: filepath.Join(c.Out, fmt.Sprintf("pass%d", j.pass))}
				for _, arm := range c.Arms {
					var r teamsolve.Result
					switch arm {
					case "single":
						r = teamsolve.Single(ctx, client, cfg, j.p)
					case "team":
						r = teamsolve.Team(ctx, client, cfg, j.p, arm)
					case "team-stress":
						g := cfg
						g.Guard = true
						r = teamsolve.TeamStress(ctx, client, g, j.p, arm, c.StressRounds)
					case "team-guard":
						g := cfg
						g.Guard = true
						r = teamsolve.Team(ctx, client, g, j.p, arm)
					}
					rw := teamRow(r, j.pass, prices)
					mu.Lock()
					results = append(results, rw)
					fmt.Fprintln(tsv, rw.tsv())
					fmt.Fprintf(os.Stderr, "  %-10s p%d %-10s solved=%-5v public=%d/%d hidden=%d/%d $%.4f %.0fs %s\n",
						r.ID, j.pass, arm, r.Solved(), r.Public.Passed, r.Public.Total, r.Hidden.Passed, r.Hidden.Total, rw.cost, r.Seconds, r.Err)
					mu.Unlock()
				}
			}
		}()
	}
	for pass := c.FirstPass; pass < c.FirstPass+c.Passes; pass++ {
		for _, p := range probs {
			jobs <- job{p, pass}
		}
	}
	close(jobs)
	wg.Wait()
	printTeamSummary(results, c.Arms)
	return nil
}

type teamRowT struct {
	r                teamsolve.Result
	pass             int
	in, out, cutoffs int64
	cost             float64
}

func (w teamRowT) tsv() string {
	e := strings.ReplaceAll(w.r.Err, "\t", " ")
	st := "-"
	if w.r.Arm == "team-stress" {
		st = "unavailable: " + w.r.Stress.Why
		if w.r.Stress.Available {
			st = fmt.Sprintf("%d/%d mismatch", w.r.Stress.Mismatches, w.r.Stress.Checked)
		}
	}
	return fmt.Sprintf("%s\t%d\t%s\t%v\t%d/%d\t%d/%d\t%s\t%d\t%d\t%d\t%d\t%.5f\t%.1f\t%s\t%s",
		w.r.ID, w.pass, w.r.Arm, w.r.Solved(), w.r.Public.Passed, w.r.Public.Total, w.r.Hidden.Passed, w.r.Hidden.Total,
		w.r.Kept, len(w.r.Calls), w.in, w.out, w.cutoffs, w.cost, w.r.Seconds, e, st)
}

func teamRow(r teamsolve.Result, pass int, prices map[string][2]float64) teamRowT {
	w := teamRowT{r: r, pass: pass}
	for _, cl := range r.Calls {
		w.in += cl.InTokens
		w.out += cl.OutTokens
		if cl.CutOff {
			w.cutoffs++
		}
		p := prices[cl.Model]
		w.cost += (float64(cl.InTokens)*p[0] + float64(cl.OutTokens)*p[1]) / 1e6
	}
	return w
}

func parsePrices(specs []string) (map[string][2]float64, error) {
	out := map[string][2]float64{}
	for _, s := range specs {
		m, io, ok := strings.Cut(s, "=")
		in, o, ok2 := strings.Cut(io, "/")
		a, e1 := strconv.ParseFloat(in, 64)
		b, e2 := strconv.ParseFloat(o, 64)
		if !ok || !ok2 || e1 != nil || e2 != nil {
			return nil, fmt.Errorf("--price %q: want model=IN/OUT", s)
		}
		out[m] = [2]float64{a, b}
	}
	return out, nil
}

// printTeamSummary prints solved counts per arm and the paired table against
// single — the comparison memdoor's bench reports (both / team only / single only).
func printTeamSummary(rows []teamRowT, arms []string) {
	by := map[string]map[string]bool{} // arm -> id#pass -> solved
	cost := map[string]float64{}
	errs := map[string]int{}
	for _, w := range rows {
		k := fmt.Sprintf("%s#%d", w.r.ID, w.pass)
		if by[w.r.Arm] == nil {
			by[w.r.Arm] = map[string]bool{}
		}
		by[w.r.Arm][k] = w.r.Solved()
		cost[w.r.Arm] += w.cost
		if w.r.Err != "" {
			errs[w.r.Arm]++
		}
	}
	fmt.Println("\narm          solved   errors   cost")
	for _, a := range arms {
		n := 0
		for _, s := range by[a] {
			if s {
				n++
			}
		}
		fmt.Printf("%-12s %3d/%-4d %4d    $%.3f\n", a, n, len(by[a]), errs[a], cost[a])
	}
	if by["single"] == nil {
		return
	}
	for _, a := range arms {
		if a == "single" || by[a] == nil {
			continue
		}
		var both, only, sonly int
		keys := make([]string, 0, len(by[a]))
		for k := range by[a] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s, ok := by["single"][k]
			if !ok {
				continue
			}
			switch t := by[a][k]; {
			case t && s:
				both++
			case t:
				only++
			case s:
				sonly++
			}
		}
		fmt.Printf("paired %-10s vs single: both %d, %s only %d, single only %d\n", a, both, a, only, sonly)
	}
}
