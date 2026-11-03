package teamsolve

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RoleBrute writes the stress-test pair: an obviously-correct brute force for
// small inputs and a random small-input generator, from the problem text alone.
const RoleBrute = "brute"

func init() { roleTemp[RoleBrute] = 0.2 }

func promptBrute(problem string) string {
	return `You write test tooling for a programming problem. You do NOT solve it efficiently.
Write two Python programs:
1. brute.py: reads the input from stdin like the real solution and prints the answer. It must be
   OBVIOUSLY correct, by exhaustive search or direct simulation of the statement, and may be
   exponentially slow: it only ever runs on tiny inputs.
2. gen.py: takes an integer seed as sys.argv[1], seeds random with it, and prints ONE valid random
   input that meets every constraint in the statement but is TINY (sizes about 1 to 6, values small),
   in exactly the input format.
Answer with the two programs in two ` + "```python" + ` blocks, brute.py first, gen.py second, nothing after.

` + problem
}

// Stress is the result of diffing a program against a brute force.
type Stress struct {
	Available  bool   // brute + gen exist and the brute passes every public sample
	Why        string // why not available
	Checked    int    // generated inputs both programs ran on
	Mismatches int
	FirstInput string
	FirstBrute string
	FirstGot   string
}

func (s Stress) Feedback() string {
	if !s.Available || s.Mismatches == 0 {
		return ""
	}
	return fmt.Sprintf(`The program passes the samples, but a brute force disagrees with it on %d of %d tiny random inputs.
First disagreement:
  input: %s
  brute force printed: %s
  program printed: %s
The brute force was written independently and checked against the samples, but it can be wrong too:
work the input by hand before deciding which one is right.`,
		s.Mismatches, s.Checked, oneLine(s.FirstInput, 300), oneLine(s.FirstBrute, 200), oneLine(s.FirstGot, 200))
}

type stressKit struct {
	brute  string
	inputs []string
	want   []string // brute's output per input
	why    string
}

// buildKit validates the brute force on the public samples, then generates
// inputs and the brute's answers once; every candidate program is diffed
// against the same kit.
func buildKit(ctx context.Context, python, reply string, p Problem, n int) stressKit {
	blocks := pyBlock.FindAllStringSubmatch(thinkBlock.ReplaceAllString(reply, ""), -1)
	if len(blocks) < 2 {
		return stressKit{why: "reply did not contain two programs"}
	}
	brute, gen := blocks[len(blocks)-2][1], blocks[len(blocks)-1][1]
	if s := RunSuite(ctx, python, brute, p.Public, 10*time.Second, false); !s.All() {
		return stressKit{why: fmt.Sprintf("brute force fails the samples (%d/%d)", s.Passed, s.Total)}
	}
	k := stressKit{brute: brute}
	seen := map[string]bool{}
	for seed := 1; seed <= n*2 && len(k.inputs) < n; seed++ {
		in := RunArgs(ctx, python, gen, "", 5*time.Second, strconv.Itoa(seed))
		if in.Kind != "" || strings.TrimSpace(in.Got) == "" || seen[in.Got] {
			continue
		}
		seen[in.Got] = true
		b := RunCase(ctx, python, brute, Case{Input: in.Got}, 5*time.Second)
		if b.Kind == "timeout" || b.Kind == "runtime error" || strings.TrimSpace(b.Got) == "" {
			continue
		}
		k.inputs = append(k.inputs, in.Got)
		k.want = append(k.want, b.Got)
	}
	if len(k.inputs) == 0 {
		k.why = "generator produced no input the brute force could answer"
	}
	return k
}

func (k stressKit) check(ctx context.Context, python, prog string) Stress {
	if k.why != "" {
		return Stress{Why: k.why}
	}
	s := Stress{Available: true}
	for i, in := range k.inputs {
		o := RunCase(ctx, python, prog, Case{Input: in}, 5*time.Second)
		s.Checked++
		if !Equal(o.Got, k.want[i]) {
			s.Mismatches++
			if s.Mismatches == 1 {
				got := o.Got
				if o.Kind == "timeout" || o.Kind == "runtime error" {
					got = "<" + o.Kind + "> " + o.Stderr
				}
				s.FirstInput, s.FirstBrute, s.FirstGot = in, k.want[i], got
			}
		}
	}
	return s
}

// TeamStress is team-guard plus a self-written stress test: the samples gate
// first; when they pass, the brute-force diff is the check; a failing check
// goes to the manager and the repair worker; a repair is kept only if it
// passes at least as many samples and disagrees with the brute force no more
// often. At most Rounds repair rounds. The brute force is written from the
// problem text alone, never shown the solution (the checker is not the author).
func TeamStress(ctx context.Context, m Chatter, c Config, p Problem, arm string, rounds int) Result {
	res := Result{ID: p.ID, Arm: arm}
	start := time.Now()
	r := &run{ctx: ctx, c: c, m: m, p: p, res: &res}
	fail := func(err error) Result {
		res.Err, res.Seconds = err.Error(), time.Since(start).Seconds()
		return res
	}
	plan, err := r.call(RolePlan, promptPlan(p.Content))
	if err != nil {
		return fail(err)
	}
	a, err := r.call(RoleIdeateA, promptIdeateA(p.Content, plan))
	if err != nil {
		return fail(err)
	}
	b, err := r.call(RoleIdeateB, promptIdeateB(p.Content, plan))
	if err != nil {
		return fail(err)
	}
	solved, err := r.call(RoleSolve, promptSolve(p.Content, plan, a, b))
	if err != nil {
		return fail(err)
	}
	best := ExtractProgram(solved)
	bestPub := RunSuite(ctx, c.Python, best, p.Public, c.PublicLimit, false)
	writeProgram(c, p.ID, arm, "solve.py", best)
	kitReply, err := r.call(RoleBrute, promptBrute(p.Content))
	if err != nil {
		return fail(err)
	}
	kit := buildKit(ctx, c.Python, kitReply, p, 40)
	bestStress := kit.check(ctx, c.Python, best)
	res.Kept = "solve"
	for round := 1; round <= rounds; round++ {
		var fb string
		switch {
		case !bestPub.All():
			fb = bestPub.Feedback()
		case bestStress.Mismatches > 0:
			fb = bestPub.Feedback() + "\n" + bestStress.Feedback()
		default:
			round = rounds + 1 // nothing left to fix
			continue
		}
		review, err := r.call(fmt.Sprintf("%s-%d", RoleReview, round), promptReview(p.Content, best, fb))
		if err != nil {
			return fail(err)
		}
		rep, err := r.call(fmt.Sprintf("%s-%d", RoleRepair, round), promptRepair(p.Content, best, review))
		if err != nil {
			return fail(err)
		}
		prog := ExtractProgram(rep)
		pub := RunSuite(ctx, c.Python, prog, p.Public, c.PublicLimit, false)
		st := kit.check(ctx, c.Python, prog)
		writeProgram(c, p.ID, arm, fmt.Sprintf("repair-%d.py", round), prog)
		if pub.Passed > bestPub.Passed || (pub.Passed == bestPub.Passed && st.Mismatches <= bestStress.Mismatches) {
			best, bestPub, bestStress = prog, pub, st
			res.Kept = fmt.Sprintf("repair-%d", round)
		}
	}
	res.Public = bestPub
	res.Stress = bestStress
	res.Hidden = RunSuite(ctx, c.Python, best, append(append([]Case{}, p.Public...), p.Hidden...), c.HiddenLimit, true)
	res.Seconds = time.Since(start).Seconds()
	writeProgram(c, p.ID, arm, "solution.py", best)
	return res
}
