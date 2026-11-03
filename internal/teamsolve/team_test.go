package teamsolve

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/justinstimatze/crystal/internal/publicai"
)

// scripted answers each role with a fixed reply.
type scripted map[string]string

func (s scripted) Chat(_ context.Context, model, _, prompt string, _ publicai.ChatOpts) (publicai.ChatResult, error) {
	role := ""
	switch {
	case strings.HasPrefix(prompt, "You are the planner"):
		role = RolePlan
	case strings.HasPrefix(prompt, "You are ideator A"):
		role = RoleIdeateA
	case strings.HasPrefix(prompt, "You are ideator B"):
		role = RoleIdeateB
	case strings.HasPrefix(prompt, "You are the solver"):
		role = RoleSolve
	case strings.HasPrefix(prompt, "You are the manager"):
		role = RoleReview
	case strings.HasPrefix(prompt, "You are the repair"):
		role = RoleRepair
	default:
		role = RoleSingle
	}
	r, ok := s[role]
	if !ok {
		panic("unexpected call: " + role)
	}
	return publicai.ChatResult{Result: publicai.Result{Text: r, Model: model}, FinishReason: "stop"}, nil
}

const doubleIt = "```python\nprint(int(input())*2)\n```"
const wrong = "```python\nprint(int(input())+2)\n```"

var prob = Problem{ID: "t", Content: "double N",
	Public: []Case{{"1\n", "2\n"}, {"5\n", "10\n"}},
	Hidden: []Case{{"7\n", "14\n"}}}

func cfg(guard bool) Config {
	return Config{Models: map[string]string{"": "m"}, Python: "python3", Guard: guard,
		PublicLimit: 5 * time.Second, HiddenLimit: 5 * time.Second}
}

func TestTeamUnguardedLetsRepairOverwriteAPassingProgram(t *testing.T) {
	m := scripted{RolePlan: "p", RoleIdeateA: "a", RoleIdeateB: "b", RoleSolve: doubleIt, RoleReview: "NO REPAIR NEEDED", RoleRepair: wrong}
	r := Team(context.Background(), m, cfg(false), prob, "team")
	if r.Err != "" || r.Kept != "repair" || r.Solved() {
		t.Fatalf("unguarded should submit the corrupted repair: %+v", r)
	}
}

func TestTeamGuardSkipsRepairWhenSamplesPass(t *testing.T) {
	m := scripted{RolePlan: "p", RoleIdeateA: "a", RoleIdeateB: "b", RoleSolve: doubleIt} // review/repair would panic
	r := Team(context.Background(), m, cfg(true), prob, "team-guard")
	if !r.Solved() || r.Kept != "solve" || len(r.Calls) != 4 {
		t.Fatalf("guard should keep the passing solve with 4 calls: %+v", r)
	}
}

func TestTeamGuardKeepsSolveWhenRepairPassesFewerSamples(t *testing.T) {
	half := "```python\nn=int(input())\nprint(2 if n==1 else 0)\n```" // passes sample 1 only
	m := scripted{RolePlan: "p", RoleIdeateA: "a", RoleIdeateB: "b", RoleSolve: half, RoleReview: "fix it", RoleRepair: wrong}
	r := Team(context.Background(), m, cfg(true), prob, "team-guard")
	if r.Kept != "solve" || r.Public.Passed != 1 {
		t.Fatalf("guard should keep the better solve: %+v", r)
	}
}

func TestTeamRepairFixesFailingSolve(t *testing.T) {
	m := scripted{RolePlan: "p", RoleIdeateA: "a", RoleIdeateB: "b", RoleSolve: wrong, RoleReview: "multiply", RoleRepair: doubleIt}
	for _, g := range []bool{false, true} {
		if r := Team(context.Background(), m, cfg(g), prob, "team"); !r.Solved() || r.Kept != "repair" {
			t.Fatalf("guard=%v: repair should be submitted and solve: %+v", g, r)
		}
	}
}

func TestSingleGradesHiddenTests(t *testing.T) {
	if r := Single(context.Background(), scripted{RoleSingle: doubleIt}, cfg(false), prob); !r.Solved() {
		t.Fatalf("%+v", r)
	}
	if r := Single(context.Background(), scripted{RoleSingle: "no code here"}, cfg(false), prob); r.Solved() || r.Public.First.Kind != "no program" {
		t.Fatalf("%+v", r.Public.First)
	}
}

func TestExtractProgramTakesLastBlockAndIgnoresThink(t *testing.T) {
	got := ExtractProgram("<think>```python\nbad\n```</think>x\n```python\na\n```\n```py\nb\n```")
	if got != "b\n" {
		t.Fatalf("%q", got)
	}
}

func TestEqualToleratesWhitespaceAndFloats(t *testing.T) {
	for _, c := range [][3]string{{"1 2\n3\n", "1 2\n3", "y"}, {"0.3333333\n", "0.33333333", "y"}, {"1.0", "1", "y"}, {"1 2", "1  3", "n"}, {"1\n2", "1", "n"}} {
		if Equal(c[0], c[1]) != (c[2] == "y") {
			t.Errorf("Equal(%q,%q) want %s", c[0], c[1], c[2])
		}
	}
}

func TestRunCaseTimeout(t *testing.T) {
	o := RunCase(context.Background(), "python3", "while True: pass\n", Case{"", ""}, 300*time.Millisecond)
	if o.Kind != "timeout" {
		t.Fatalf("%+v", o)
	}
}
