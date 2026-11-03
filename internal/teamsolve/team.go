package teamsolve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/justinstimatze/crystal/internal/publicai"
)

// Chatter is the one model call a role makes. *publicai.Client satisfies it;
// tests pass a scripted one.
type Chatter interface {
	Chat(ctx context.Context, model, system, prompt string, o publicai.ChatOpts) (publicai.ChatResult, error)
}

// Roles, in the order the team runs them.
const (
	RoleSingle  = "single"
	RolePlan    = "plan"
	RoleIdeateA = "ideate-a"
	RoleIdeateB = "ideate-b"
	RoleSolve   = "solve"
	RoleReview  = "review"
	RoleRepair  = "repair"
)

// Temperatures per role: GVS5H's (plan 0.3, ideation 0.4, manager and worker 0.2),
// per memdoor's parity notes. The single call samples like the solver.
var roleTemp = map[string]float64{
	RoleSingle: 0.2, RolePlan: 0.3, RoleIdeateA: 0.4, RoleIdeateB: 0.4,
	RoleSolve: 0.2, RoleReview: 0.2, RoleRepair: 0.2,
}

// Config shapes one arm on one problem.
type Config struct {
	Models    map[string]string // role -> model; "" key is the default
	MaxTokens int64
	TopP      float64
	NoThink   bool
	Python    string
	// Guard turns on the two fixes memdoor's NOTES name for the method's losses
	// and never ran: skip the repair call when every sample already passed
	// (memdoor's loop still asks the model to re-echo the program, which can
	// corrupt it), and keep the solver's program if the repaired one passes
	// fewer samples (never let a later round overwrite a better program).
	Guard        bool
	Sample       int
	PublicLimit  time.Duration
	HiddenLimit  time.Duration
	TranscriptTo string // dir for every prompt and reply, "" = none
}

func (c Config) model(role string) string {
	if m := c.Models[role]; m != "" {
		return m
	}
	return c.Models[""]
}

// Call is one model call's record.
type Call struct {
	Role, Model                 string
	InTokens, OutTokens         int64
	LatencyMS                   int64
	FinishReason                string
	Cached, CutOff, NoCodeFound bool
}

// Result is one arm on one problem.
type Result struct {
	ID, Arm string
	Public  Suite
	Hidden  Suite
	Calls   []Call
	Seconds float64
	Kept    string // which program was submitted: solve | repair | single
	Stress  Stress // team-stress only
	Err     string
}

// Solved means every hidden and public test passed.
func (r Result) Solved() bool { return r.Err == "" && r.Hidden.All() }

type run struct {
	ctx context.Context
	c   Config
	m   Chatter
	p   Problem
	mu  sync.Mutex
	res *Result
}

func (r *run) call(role, prompt string) (string, error) {
	model := r.c.model(role)
	if model == "" {
		return "", fmt.Errorf("no model for role %s", role)
	}
	cr, err := r.m.Chat(r.ctx, model, "", prompt, publicai.ChatOpts{
		Temperature: tempFor(role), TopP: r.c.TopP, MaxTokens: r.c.MaxTokens, Sample: r.c.Sample, NoThink: r.c.NoThink,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", role, err)
	}
	r.mu.Lock()
	r.res.Calls = append(r.res.Calls, Call{Role: role, Model: model, InTokens: cr.InputTokens, OutTokens: cr.OutputTokens,
		LatencyMS: cr.LatencyMS, FinishReason: cr.FinishReason, Cached: cr.Cached, CutOff: cr.FinishReason == "length"})
	r.mu.Unlock()
	if r.c.TranscriptTo != "" {
		d := filepath.Join(r.c.TranscriptTo, r.p.ID, r.res.Arm)
		_ = os.MkdirAll(d, 0o755)
		_ = os.WriteFile(filepath.Join(d, role+".prompt.md"), []byte(prompt), 0o644)
		_ = os.WriteFile(filepath.Join(d, role+".reply.md"), []byte(cr.Text), 0o644)
	}
	return cr.Text, nil
}

var pyBlock = regexp.MustCompile("(?s)```(?:python|py)?[ \\t]*\\n(.*?)```")
var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// ExtractProgram takes the last ```python block of a reply, as memdoor's
// extract.py does. No block → "" (memdoor falls back to the whole reply; a
// prose reply is not a program, and running it only adds a syntax error).
func ExtractProgram(reply string) string {
	reply = thinkBlock.ReplaceAllString(reply, "")
	m := pyBlock.FindAllStringSubmatch(reply, -1)
	if len(m) == 0 {
		return ""
	}
	return strings.TrimSpace(m[len(m)-1][1]) + "\n"
}

// Single is the one-call baseline.
func Single(ctx context.Context, m Chatter, c Config, p Problem) Result {
	res := Result{ID: p.ID, Arm: "single", Kept: "single"}
	start := time.Now()
	r := &run{ctx: ctx, c: c, m: m, p: p, res: &res}
	reply, err := r.call(RoleSingle, promptSingle(p.Content))
	if err != nil {
		res.Err = err.Error()
		return res
	}
	prog := ExtractProgram(reply)
	res.Public = RunSuite(ctx, c.Python, prog, p.Public, c.PublicLimit, false)
	res.Hidden = RunSuite(ctx, c.Python, prog, append(append([]Case{}, p.Public...), p.Hidden...), c.HiddenLimit, true)
	res.Seconds = time.Since(start).Seconds()
	writeProgram(c, p.ID, res.Arm, "solution.py", prog)
	return res
}

// Team is the GVS5H team as memdoor's team-llm-lcb runs it: plan → (ideate-a ∥
// ideate-b) → solve → samples → review → repair → samples. One round.
func Team(ctx context.Context, m Chatter, c Config, p Problem, arm string) Result {
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
	var a, b string
	var ea, eb error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a, ea = r.call(RoleIdeateA, promptIdeateA(p.Content, plan)) }()
	go func() { defer wg.Done(); b, eb = r.call(RoleIdeateB, promptIdeateB(p.Content, plan)) }()
	wg.Wait()
	if ea != nil {
		return fail(ea)
	}
	if eb != nil {
		return fail(eb)
	}
	solved, err := r.call(RoleSolve, promptSolve(p.Content, plan, a, b))
	if err != nil {
		return fail(err)
	}
	prog1 := ExtractProgram(solved)
	pub1 := RunSuite(ctx, c.Python, prog1, p.Public, c.PublicLimit, false)
	writeProgram(c, p.ID, arm, "solve.py", prog1)

	final, finalPub, kept := prog1, pub1, "solve"
	if !(c.Guard && pub1.All()) {
		review, err := r.call(RoleReview, promptReview(p.Content, prog1, pub1.Feedback()))
		if err != nil {
			return fail(err)
		}
		rep, err := r.call(RoleRepair, promptRepair(p.Content, prog1, review))
		if err != nil {
			return fail(err)
		}
		prog2 := ExtractProgram(rep)
		pub2 := RunSuite(ctx, c.Python, prog2, p.Public, c.PublicLimit, false)
		writeProgram(c, p.ID, arm, "repair.py", prog2)
		if !c.Guard || pub2.Passed >= pub1.Passed {
			final, finalPub, kept = prog2, pub2, "repair"
		}
	}
	res.Kept, res.Public = kept, finalPub
	res.Hidden = RunSuite(ctx, c.Python, final, append(append([]Case{}, p.Public...), p.Hidden...), c.HiddenLimit, true)
	res.Seconds = time.Since(start).Seconds()
	writeProgram(c, p.ID, arm, "solution.py", final)
	return res
}

func writeProgram(c Config, id, arm, name, prog string) {
	if c.TranscriptTo == "" {
		return
	}
	d := filepath.Join(c.TranscriptTo, id, arm)
	_ = os.MkdirAll(d, 0o755)
	_ = os.WriteFile(filepath.Join(d, name), []byte(prog), 0o644)
}

// tempFor maps a numbered role ("review-2") to its base role's temperature.
func tempFor(role string) float64 {
	if t, ok := roleTemp[role]; ok {
		return t
	}
	if i := strings.LastIndex(role, "-"); i > 0 {
		return roleTemp[role[:i]]
	}
	return 0.2
}
