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

## The question memdoor's headline asks: a cheap team vs one expensive call

memdoor: "Five Qwen3.8-27B agents beat Fable 5 on LiveCodeBench Hard." Its Fable 5 numbers are the
GVS5H authors' files, not its own run. Those files (`runs/patched_verdicts.json`, five passes per
arm, every problem) cover all 73 of our problems, so the claim can be checked per problem, and one
frontier single call was run here for comparison.

| on the same problems | first 10 (memdoor's set) | our 73 AtCoder | all 100 |
|---|---|---|---|
| Fable 5, one call, thinking on, 128K (authors, mean of 5) | 8.0 | 65.6 | 90.4 |
| Qwen3.8-27B team, thinking on, 128K, up to 10 rounds (authors, mean of 5) | 9.6 | 66.0 | 92.4 |
| paired over 5 passes: team-only / Fable-only, sign test | 10 / 2, p = 0.04 | 10 / 8, p = 0.81 | 21 / 11, p = 0.11 |
| **Claude Opus 5.5, one call, effort high (here, 1 pass)** | | **64** | |

Per problem on the 73: Fable 5 ~$0.75 (authors' mean 14.7K output tokens at $10/$50), the Qwen team
~$0.48 (192K output tokens at $0.20/$2.50; memdoor measured a median $0.39 and 40 minutes a run),
Opus 5.5 **$0.08** (3.8K output tokens, $5.85 for all 73, median ~30 s), regraded uncontended, no
errors, no cut-offs.

- **"Beat" holds on memdoor's 10 problems only.** On all 100 the team leads by 2 (p = 0.11); on the
  73 AtCoder problems it is a tie. The first ten are the newest contests (least contamination risk),
  so a team edge concentrated there would be interesting, but ten problems cannot establish it.
  memdoor's own README body says "parity with their harness".
- **One Opus 5.5 call lands within about two problems of both, at a sixth of the team's cost and a
  tenth of Fable's, in minutes rather than most of an hour.** On this benchmark the cheaper frontier
  model removes most of the cost argument for the cheap team. One pass; contamination not ruled out.
- **Thinking, not the team, is most of the climb**, in the authors' data: Qwen single call ~25 of 73
  thinking off (ours) → ~50 thinking on (authors) → 66 as a 10-round team.

### A failed replication attempt, recorded so it is not repeated

Qwen3.8-27B thinking on through the Hugging Face router, our one-round team: single 29, team 50 of
the 70 problems finished before the run was stopped at the 2-hour limit. **Invalid as a
replication**: the provider the router picked silently caps output at exactly 40,960 tokens
whatever `max_tokens` says, and 47 of 72 single calls were cut there (the authors' mean single
call is 107K tokens). Pin a provider with a long output limit (`model:provider` on the router) or
self-host before comparing thinking-on numbers.

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

## Pre-registered for stage B (written 2026-11-03, before any stage-B run)

Two tracks, judged separately, so a slow win cannot pass as a cheap one.

**Interactive track (the primary claim).** An open-only setup (EU models preferred: Mistral, Apertus)
passes if, on fresh problems released after the models' training cutoffs, over 3 passes with paired
tests:
- accuracy ≥ Fable 5.1 single call (confidence-interval lower bound not below it), at ≤ 1/3 of its
  cost per problem;
- **and** wall-clock per problem p50 ≤ 2× and p90 ≤ 5× the frontier single call's on the same set.
Fallback claim: the same bar met on most task categories, categories fixed before the run.
Comparators: Fable 5.1 and Opus 5.5 single calls at high effort. Grading separate and uncontended;
every slice reported; provider caps and errors logged.

**Batch track (a separate path, not a fallback).** No latency bar; the bar is accuracy at cost,
using batch pricing where it exists for every arm (frontier batch APIs included, so the comparison
stays fair). Candidates for this track are work that can wait: overnight review, test writing,
long-horizon reasoning like the weight-interpretation example (a local 27B taking about an hour a
prompt). Reported on its own table, never merged into the interactive verdict.

## Stage A screen: EU open models with reasoning on (2026-11-04)

Single call, `reasoning_effort: high`, same 73 problems, regraded uncontended.

| model | solved | Opus 5.5 same set | spend | median / max time per problem |
|---|---|---|---|---|
| Mistral Large 4 (1T, ~52B active), 128K cap | 52 (5 calls lost to dropped connections) | 64 | ~$18.50 | ~7 min / 40 min |
| Mistral Small 4 (119B, Apache 2.0), 64K cap | 31 | 64 | $1.28 | 2.3 min / ~9 min |

- **Correction to every earlier Mistral number:** Mistral's API reasons only when `reasoning_effort`
  is set; the earlier runs sent none and got 0 reasoning tokens. Medium 3.5's 10-17 was its
  thinking-off score.
- At a 64K cap, 21 of 71 Large 4 calls were cut off (40 solved); a 128K cap recovered 12.
- Gate (within ~10 of Opus 5.5): Large 4 borderline on accuracy, Small 4 fails.
- Interactive bar: Large 4 through the API is ~14× Opus 5.5's median time; it can only compete on
  the batch track. Cost: its single call (~$0.25) already costs about a third of Fable 5's, so a
  Large 4 team undercuts Fable only when self-hosted at high utilisation.
