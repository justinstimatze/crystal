package teamsolve

// The role prompts, verbatim from memdoor's team-llm-lcb and single-lcb steps
// (Apache-2.0), with the {{ output "…" }} template references filled in Go.

func promptSingle(problem string) string {
	return `Solve this programming problem. Answer with one complete Python program in a
single ` + "```python" + ` code block and nothing after it. If the problem gives starter
code, complete it; otherwise read stdin and print the answer.

` + problem
}

func promptPlan(problem string) string {
	return `You are the planner of a small team solving a programming problem. Nobody on the
team shares your context: everything they will know is what you write here.
Write a plan: restate the problem precisely (input, output, constraints), the
key observations, two or three candidate approaches with their time and space
complexity and the risks of each, the edge cases, and what the solver must verify.
Do not write the final code.

` + problem
}

func promptIdeateA(problem, plan string) string {
	return `You are ideator A on a team. Read the problem and the planner's plan below and
develop ONE concrete proposal, the most direct approach you can prove correct,
within the time limit. Give the invariant, a correctness argument, the complexity,
pseudocode close to Python, and the edge cases it must handle. Do not write the
final program.

# Problem
` + problem + `

# Plan
` + plan
}

func promptIdeateB(problem, plan string) string {
	return `You are ideator B on a team. Read the problem and the planner's plan below and
develop ONE concrete proposal that differs from the obvious approach, or the
obvious approach argued carefully with its proof. Look for the trap in the problem:
the constraint, the case or the overflow the direct method misses. Give the
invariant, a correctness argument, the complexity, pseudocode close to Python, and
the edge cases. Do not write the final program.

# Problem
` + problem + `

# Plan
` + plan
}

func promptSolve(problem, plan, a, b string) string {
	return `You are the solver. Read the problem, the plan and the two proposals, choose the
approach you can argue is correct and fast enough, and write the complete program.
Answer with a short justification, then ONE ` + "```python" + ` block with the whole program
and nothing after it. If the problem gives starter code, complete it exactly;
otherwise read stdin and print the answer. No input() prompts, no debug output.

# Problem
` + problem + `

# Plan
` + plan + `

# Proposal A
` + a + `

# Proposal B
` + b
}

func promptReview(problem, program, testResult string) string {
	return `You are the manager. Below are the problem, the solver's program and the result of
running it on the sample tests. If every sample passed, answer exactly:
NO REPAIR NEEDED
and nothing else. Otherwise diagnose the failure from the test output (wrong answer,
crash, timeout, output format) and write precise repair instructions for a fresh
worker who will see only this review and the program: what is wrong, where, and what
to change. Do not write the whole program.

# Problem
` + problem + `

# Program
` + "```python\n" + program + "```" + `

# Sample test result
` + testResult
}

func promptRepair(problem, program, review string) string {
	return `You are the repair worker. If the review says NO REPAIR NEEDED, answer with the
program below unchanged in ONE ` + "```python" + ` block. Otherwise apply the review's
instructions and answer with the corrected complete program in ONE ` + "```python" + ` block
and nothing after it.

# Problem
` + problem + `

# Program
` + "```python\n" + program + "```" + `

# Review
` + review
}
