// Package teamsolve runs the GVS5H "team" method (Lee, 2026) — planner, two
// ideators, solver, manager, repair worker, each a single model call from a
// fresh context — against a one-call baseline, on LiveCodeBench problems, with
// the problem's own sample tests as the only success signal the loop sees and
// the hidden tests as the score. The role prompts are the ones memdoor's
// team-llm-lcb bench uses (github.com/guregodevo/memdoor, Apache-2.0,
// examples/workflows/team-solve/bench/team-llm-lcb/steps), so a run here is
// comparable to memdoor's published 47-vs-35 on Qwen3.8-27B.
//
// crystal's angle: the gain memdoor measures comes from a cheap executable
// verifier, not from the models. This package asks whether that holds when the
// team is open-source (Apertus) instead of open-weight (Qwen), and whether two
// guards memdoor's NOTES name but never tested — keep the best program, skip a
// repair nobody asked for — move the number.
package teamsolve

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Case is one stdin/stdout test.
type Case struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// Problem is one stdin-style LiveCodeBench problem.
type Problem struct {
	ID      string
	Content string
	Public  []Case
	Hidden  []Case
}

type lcbRow struct {
	QuestionID      string `json:"question_id"`
	QuestionContent string `json:"question_content"`
	Platform        string `json:"platform"`
	StarterCode     string `json:"starter_code"`
	Public          []struct {
		Case
		TestType string `json:"testtype"`
	} `json:"public_test_cases"`
	Private []struct {
		Case
		TestType string `json:"testtype"`
	} `json:"private_test_cases"`
}

// Load reads the jsonl that bench/teamsolve/lcb_filter.py writes and keeps the
// stdin problems (AtCoder). LeetCode's functional problems need a call harness
// this package does not have yet; they are skipped and counted.
func Load(path string) (probs []Problem, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	for sc.Scan() {
		var r lcbRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, 0, fmt.Errorf("%s: %w", path, err)
		}
		if r.StarterCode != "" || len(r.Public) == 0 || r.Public[0].TestType != "stdin" {
			skipped++
			continue
		}
		p := Problem{ID: r.QuestionID, Content: r.QuestionContent}
		for _, c := range r.Public {
			p.Public = append(p.Public, c.Case)
		}
		for _, c := range r.Private {
			p.Hidden = append(p.Hidden, c.Case)
		}
		probs = append(probs, p)
	}
	sort.Slice(probs, func(i, j int) bool { return probs[i].ID < probs[j].ID })
	return probs, skipped, sc.Err()
}
