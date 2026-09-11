package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/justinstimatze/crystal/internal/record"
	"github.com/justinstimatze/crystal/internal/step"
	"github.com/justinstimatze/crystal/internal/transcript"
)

// StepsCmd measures the DECOMPOSITION PROFILE of real frontier work: for
// every move the model made, how much of it was already recoverable from
// state the model could see. This is the substrate lifted from the tool
// call (whose output is deterministic given its input, and therefore never
// the interesting variable) to the STEP — what the model chose to do next.
//
// Deterministic, no model calls. The output is a ranked table of chore
// shapes by how mechanical they are, which is the candidate list for
// shifting work off the frontier tier.
type StepsCmd struct {
	Home    []string `help:"Home dirs to scan (transcripts at <home>/.claude/projects/*/*.jsonl). Repeatable." required:""`
	Project string   `help:"Only transcripts whose project dir contains this token."`
	Top     int      `help:"Show at most this many transition shapes." default:"15"`
	Min     int      `help:"A transition shape needs at least this many steps to be reported." default:"20"`
	Samples int      `help:"Print this many example moves per reported shape." default:"0"`
}

// shape is a transition between two tools — the coarsest chore signature
// that is free from the trace (the model drew these boundaries itself).
type shape struct {
	from, to string
}

// Run walks transcripts, builds steps, and reports the profile.
func (c *StepsCmd) Run() error {
	var files []string
	for _, home := range c.Home {
		matches, _ := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", "*.jsonl"))
		files = append(files, matches...)
	}
	if len(files) == 0 {
		return usageError{fmt.Errorf("no transcripts found under the given --home dirs")}
	}

	// Stream one transcript at a time. A step never spans two sessions and
	// one file IS one session, so per-file processing is exactly equivalent
	// to walking the whole corpus at once — and holds one transcript in
	// memory instead of every tool result across thousands of them.
	var p step.Profile
	rp := step.NewRunProfile()
	byShape := map[shape]*step.Profile{}
	samples := map[shape][]step.Step{}
	var dropped, scanned, calls, nsteps int

	for _, f := range files {
		if c.Project != "" && !strings.Contains(filepath.Base(filepath.Dir(f)), c.Project) {
			continue
		}
		st := step.NewStreamer(func(s step.Step) {
			nsteps++
			p.Add(s)
			rp.Add(s)
			sh := shape{s.Prev.Tool, s.Next.Tool}
			sp := byShape[sh]
			if sp == nil {
				sp = &step.Profile{}
				byShape[sh] = sp
			}
			sp.Add(s)
			if c.Samples > 0 && len(samples[sh]) < c.Samples {
				s.Prev.Result = record.Output{} // drop the bulky result body
				s.Next.Result = record.Output{}
				samples[sh] = append(samples[sh], s)
			}
		})
		d, err := transcript.StreamFile(f, func(r record.Record) {
			calls++
			st.Push(r)
		})
		dropped += d
		if err != nil && calls == 0 {
			continue
		}
		scanned++
	}
	if nsteps == 0 {
		return usageError{fmt.Errorf("no steps built from %d transcripts", scanned)}
	}
	rp.Flush()

	fmt.Printf("crystal steps: %d transcripts → %d tool calls → %d steps (%d records dropped)\n",
		scanned, calls, nsteps, dropped)
	fmt.Printf("(a step is (result of call k) → (the model's call k+1), within one turn; deterministic, no model)\n\n")

	fmt.Printf("DECOMPOSITION PROFILE (all steps, n=%d)\n", p.N)
	for _, cl := range []step.Class{step.Repeat, step.Derived, step.Partial, step.Authored, step.Bare} {
		n := p.Counts[cl]
		fmt.Printf("  %-9s %6d  %5.1f%%  %s\n", cl, n, pct(n, p.N), classGloss(cl))
	}
	fmt.Printf("\n  shiftable ceiling : %.1f%%  (repeat + derived — information was already visible)\n", p.Shiftable()*100)
	fmt.Printf("  judgment residual : %.1f%%  (authored — the model supplied text found nowhere in prior state)\n",
		pct(p.Counts[step.Authored], p.N))
	fmt.Printf("  mean derived frac : %.2f\n", p.MeanFrac)

	// A step is too small a unit to hand a cheap executor — nobody delegates
	// "the next Read." The candidate unit above it is an EPISODE: a maximal
	// mechanical run bounded by two Authored (judgment) steps. These two
	// numbers test that hypothesis directly, deterministically, from the
	// same classified steps: does judgment show up as one decision at a
	// time (isolated), or in multi-step stretches an episode boundary would
	// cut through?
	isolated := rp.IsolatedFrac(step.Authored)
	gapN, gapSum := rp.GapCount()
	fmt.Printf("\nEPISODE HYPOTHESIS (is a step too small a unit?)\n")
	fmt.Printf("  isolated authored steps : %.1f%%  (fraction of authored RUNS that are length 1)\n", isolated*100)
	if gapN > 0 {
		fmt.Printf("  mean episode interior   : %.1f mechanical steps  (n=%d bounded gaps, %d steps total)\n",
			rp.MeanGap(), gapN, gapSum)
	} else {
		fmt.Printf("  mean episode interior   : n/a (no gap bounded by two authored steps)\n")
	}

	// Per-transition-shape breakdown, ranked by shiftable volume: the
	// candidates worth building a verified substitution for are the ones
	// that are both mechanical AND frequent.
	type row struct {
		sh shape
		p  step.Profile
	}
	var rows []row
	for sh, sp := range byShape {
		if sp.N < c.Min {
			continue
		}
		rows = append(rows, row{sh, *sp})
	}
	sort.Slice(rows, func(i, j int) bool {
		li := rows[i].p.Shiftable() * float64(rows[i].p.N)
		lj := rows[j].p.Shiftable() * float64(rows[j].p.N)
		if li != lj {
			return li > lj
		}
		return rows[i].p.N > rows[j].p.N
	})

	fmt.Printf("\nBY TRANSITION SHAPE (>=%d steps), ranked by shiftable VOLUME (steps removable = shiftable x n)\n", c.Min)
	fmt.Printf("  %-22s %7s %9s %9s %9s\n", "shape", "n", "shiftable", "authored", "removable")
	for i, r := range rows {
		if c.Top > 0 && i >= c.Top {
			break
		}
		fmt.Printf("  %-22s %7d %8.1f%% %8.1f%% %9.0f\n",
			r.sh.from+" → "+r.sh.to, r.p.N, r.p.Shiftable()*100,
			pct(r.p.Counts[step.Authored], r.p.N), r.p.Shiftable()*float64(r.p.N))
		if c.Samples > 0 {
			printSamples(samples[r.sh], c.Samples)
		}
	}

	fmt.Printf("\nReading this honestly: 'shiftable' means the information was ALREADY VISIBLE to the\n")
	fmt.Printf("model, so a harness plus a cheap executor plausibly could have produced the move. It is\n")
	fmt.Printf("a CEILING, not a result — the selection rule is still missing, and no verifier has yet\n")
	fmt.Printf("confirmed any specific substitution. The authored column is the judgment residual that\n")
	fmt.Printf("stays on the frontier tier no matter how good the harness gets.\n")
	return nil
}

func printSamples(ss []step.Step, n int) {
	shown := 0
	for _, s := range ss {
		if shown >= n {
			return
		}
		args := ""
		for _, a := range firstArgs(s.Next) {
			args = a
			break
		}
		fmt.Printf("      · %-8s %-52s %s\n", s.Class, stepTrunc(args, 52), stepTrunc(s.Prev.Tool+" result", 20))
		shown++
	}
}

func firstArgs(r record.Record) []string {
	var out []string
	for k, v := range r.Args {
		if s, ok := v.(string); ok {
			out = append(out, k+"="+s)
		}
	}
	sort.Strings(out)
	return out
}

func classGloss(c step.Class) string {
	switch c {
	case step.Repeat:
		return "this exact move already happened — pure memoization"
	case step.Derived:
		return "every argument was verbatim in visible state — clipboard work"
	case step.Partial:
		return "some arguments visible, some authored"
	case step.Authored:
		return "no argument text was in prior state — judgment residual"
	case step.Bare:
		return "no substantive argument text to attribute"
	}
	return ""
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

func stepTrunc(s string, n int) string {
	s = sanitizeLine(s)
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func sanitizeLine(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		out = append(out, r)
	}
	return string(out)
}
