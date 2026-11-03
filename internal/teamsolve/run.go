package teamsolve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Outcome of running a program on one test.
type Outcome struct {
	Pass     bool
	Got      string
	Kind     string // "", "wrong answer", "runtime error", "timeout", "no program"
	Stderr   string
	Duration time.Duration
}

// RunArgs runs code with argv and stdin, for generators.
func RunArgs(ctx context.Context, python, code, stdin string, timeout time.Duration, args ...string) Outcome {
	return runPy(ctx, python, code, stdin, timeout, args)
}

// RunCase runs code under python3 -I with the case's input on stdin.
func RunCase(ctx context.Context, python, code string, c Case, timeout time.Duration) Outcome {
	o := runPy(ctx, python, code, c.Input, timeout, nil)
	if o.Kind == "" {
		if Equal(o.Got, c.Output) {
			o.Pass = true
		} else {
			o.Kind = "wrong answer"
		}
	}
	return o
}

// runPy runs code; Kind is "" on a clean exit (the caller judges the output).
func runPy(ctx context.Context, python, code, stdin string, timeout time.Duration, args []string) Outcome {
	if strings.TrimSpace(code) == "" {
		return Outcome{Kind: "no program"}
	}
	dir, err := os.MkdirTemp("", "teamsolve-*")
	if err != nil {
		return Outcome{Kind: "runtime error", Stderr: err.Error()}
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "solution.py")
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		return Outcome{Kind: "runtime error", Stderr: err.Error()}
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, python, append([]string{"-I", src}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	start := time.Now()
	err = cmd.Run()
	o := Outcome{Got: out.String(), Stderr: tail(errb.String(), 400), Duration: time.Since(start)}
	switch {
	case errors.Is(cctx.Err(), context.DeadlineExceeded):
		o.Kind = "timeout"
	case err != nil:
		o.Kind = "runtime error"
	}
	return o
}

// Suite is a program's result on a list of tests.
type Suite struct {
	Passed, Total int
	First         *Outcome // first failure
	FirstCase     *Case
}

func (s Suite) All() bool { return s.Total > 0 && s.Passed == s.Total }

// RunSuite runs every case; stopEarly stops at the first failure (hidden-test
// grading: a timeout would otherwise cost the limit on each of forty tests for
// the same verdict, the same shortcut memdoor's grade.py --private takes).
func RunSuite(ctx context.Context, python, code string, cases []Case, timeout time.Duration, stopEarly bool) Suite {
	s := Suite{Total: len(cases)}
	for i := range cases {
		o := RunCase(ctx, python, code, cases[i], timeout)
		if o.Pass {
			s.Passed++
			continue
		}
		if s.First == nil {
			oc, cc := o, cases[i]
			s.First, s.FirstCase = &oc, &cc
		}
		if stopEarly {
			break
		}
	}
	return s
}

// Feedback renders a public-sample result the way a manager reads it.
func (s Suite) Feedback() string {
	if s.All() {
		return fmt.Sprintf("All %d sample tests passed.", s.Total)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d/%d sample tests passed.\n", s.Passed, s.Total)
	if s.First != nil {
		fmt.Fprintf(&b, "First failing sample: %s\n  input: %s\n  expected: %s\n  got: %s\n",
			s.First.Kind, oneLine(s.FirstCase.Input, 300), oneLine(s.FirstCase.Output, 200), oneLine(s.First.Got, 200))
		if s.First.Stderr != "" {
			fmt.Fprintf(&b, "  stderr: %s\n", oneLine(s.First.Stderr, 300))
		}
	}
	return b.String()
}

// Equal compares outputs the way LiveCodeBench's stdin checker does in effect:
// line by line, whitespace-trimmed, with numeric tokens compared as numbers
// (so "1.0" matches "1" and float answers within 1e-6 match).
func Equal(got, want string) bool {
	g, w := lines(got), lines(want)
	if len(g) != len(w) {
		return false
	}
	for i := range g {
		if g[i] == w[i] {
			continue
		}
		gt, wt := strings.Fields(g[i]), strings.Fields(w[i])
		if len(gt) != len(wt) {
			return false
		}
		for j := range gt {
			if gt[j] == wt[j] {
				continue
			}
			a, e1 := strconv.ParseFloat(gt[j], 64)
			b, e2 := strconv.ParseFloat(wt[j], 64)
			if e1 != nil || e2 != nil {
				return false
			}
			d := a - b
			if d < 0 {
				d = -d
			}
			if d > 1e-6*max(1, abs(b)) {
				return false
			}
		}
	}
	return true
}

func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func oneLine(s string, n int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", `\n`)
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}

func tail(s string, n int) string {
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
