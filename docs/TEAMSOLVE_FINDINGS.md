# team-solve findings (2026-11-03)

What actually works when a small team of open-model calls takes on hard programming problems,
measured against one call on the same problems. Probe: `crystal team-solve` (memdoor's port of the
GVS5H method, its prompts verbatim), 73 AtCoder stdin problems from GVS5H's 100-problem
LiveCodeBench list (contests 2024-12 to 2025-04; the 27 LeetCode problems need a call harness and
are skipped). The loop sees only the public samples; the score is every hidden test. Every row,
prompt, reply and program was kept locally; numbers below are after `crystal team-regrade`
(uncontended re-grading; see "Grading noise"). Reproduce with `bench/teamsolve/README.md`,
`summarize.py` and `analyze.py`.

## Results (solved of 73)

| model | pass | single | team | team-guard | team-stress | cascade (single, else team) |
|---|---|---|---|---|---|---|
| Qwen3.8-27B, thinking off | 1 | 27 | 34 | 34 | 35 | **37** |
| Qwen3.8-27B, thinking off | 2 | 24* | 32* | 34* | | 31 |
| Mistral Medium 3.5 | 1 | 10 | 17 | 17 | | **19** |
| Mistral Medium 3.5 | 2 | 14 | 17 | 17 | 17 | **22** |
| Apertus 1.5 70B | 1 | 2 | 3 | 3 | | |

\* Qwen pass 2 lost rows to Hugging Face router 504s (single 4, team 10, team-guard 2), counted
unsolved; the team/team-guard gap there is errors, not method.

Cost per problem (Qwen pass 1): single $0.008, team $0.039, cascade $0.031. Mistral: single
$0.026-0.031, team ~$0.10, cascade ~$0.10. team-guard and team-stress share team's cached first
calls, so their costs are notional.

## What it says

1. **The team beats one call, modestly and consistently.** Paired over both Qwen passes
   (team-guard vs single): 24 problems only the team solved, 7 only the single call (sign test
   p ≈ 0.003). Mistral, both passes: 20 vs 10 (p ≈ 0.10). About +7 problems in 73 at 4-5x the
   cost, in line with memdoor's 47 vs 35 of 100. Apertus 1.5 70B is at the floor on these problems
   and says nothing either way.
2. **The gain is in planning and ideation, not repair.** The solver's first program passes the
   samples far more often in the team (Qwen 36 → 54, Mistral 22 → 39 programs); the review/repair
   round adds 2-4 problems. Consistent with Olausson et al. (ICLR 2024): diversity of initial
   programs, not repair, is where the budget pays.
3. **Routing on the samples beats always-team.** Run one call; submit it if it passes every public
   sample; otherwise run the team. Better than always-team in 4 of 5 comparisons (Qwen 37 vs 34,
   Mistral 19 vs 17 and 22 vs 17) at equal or lower cost. Why: a single call that passes the
   samples is more often right on the hidden tests (Qwen 75%) than a team program that does (63%).
   Post hoc on these runs, so it needs a fresh pass to confirm, but it was the literature's
   prediction (escalate on verification failure) before it was looked at.
4. **The samples are a weak verifier, and the obvious fix did not help.** 25-56% of programs
   passing every sample fail hidden tests. `team-stress` (a brute force and tiny-input generator
   written from the problem text alone, the brute force required to pass the samples, the solution
   diffed on 40 inputs) moved Qwen 34 → 35 and Mistral 17 → 17. The brute force failed the samples
   on 26-29 of 73 problems, so the check was not even available there. LLM-written tests share
   the model's misreadings (Ma et al. 2025; TestCase-Eval).
5. **Mixing families adds nothing here.** Every problem Mistral solved in any arm or pass, Qwen
   also solved (28 shared, 14 Qwen only, 0 Mistral only). A Mistral member could not raise a
   Qwen team's ceiling on this set.
6. **The guards are inert.** Skipping a repair nobody asked for and keeping the better program
   changed no outcome in any run: in a one-round loop the repair rarely makes a passing program
   worse. (memdoor's losses came from its 10-round loop.)
7. **The reply cap matters for Mistral.** At 8,192 tokens, Mistral ran away inside code comments on
   13 single calls; at 32,768 those runaways still solved nothing, so the cap was not hiding wins,
   but a capped single call should be reported as such.

## Grading noise (fixed)

Grading inside a live run shares 4 CPUs with every problem in flight. The same program passed
43/43 hidden tests in one run and 3/43 in another (Mistral pass 2, abc391_f): slow but correct
Python timing out under load. `team-regrade` re-runs every saved program uncontended; it changed
4 verdicts across 9 run directories (all in the two runs graded while three runs overlapped).
Grade separately from generation.

## Not established

- Every comparison is one or two passes; p-values from 0.003 (Qwen pooled) to 0.6 (Mistral pass 2).
- Contamination: contests predate the models' cutoffs.
- N independent single calls plus execution-based selection at the team's budget (AlphaCode/CodeT/
  S*-style) is untested and is the comparison that decides whether "team" is the right shape at
  all; the literature favours it.

## Related, same day

freshet `cmd/gatewrite`: asked to write the deterministic gate that freshet's hand gate encodes,
Mistral Medium 3.5 and Apertus 1.5 70B write questions about distance and weather, the battery's
own trap, and land at the bare model (47 and 40 of 65 vs the hand gate's 65 and 60). The same
lesson as point 4: a verifier written by the model inherits the model's blind spot.
