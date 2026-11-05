# Plan: testing the most promising neuroscience-inspired ideas, efficiently (2026-11-04)

Written at the round-1 plateau. Pre-registered here before any of these runs.

## Where we are (round 1, 118 older LiveCodeBench AtCoder problems, hidden tests, regraded)
- DeepSeek V4.1-Flash alone 99/118 at $0.011/problem; GLM-5.3-Flash 76; gpt-oss-120b 63 ($0.034).
- Perfect routing over the three: 101. gpt-oss rescues 0 of DeepSeek's 19 failures; GLM rescues 2.
- Cascade DeepSeek > GLM: 100 at $0.014. The length-based evidence-threshold rule lost everywhere.
- Best correctness signal: passes samples AND others pass (AUROC 0.943). False passes: 17 of 255.
- Lesson already: phi correlation picked the wrong recruit (gpt-oss, least correlated, rescues nothing).
  The quantity that matters is the **rescue rate**: P(candidate solves | team fails).

## The efficiency principle
Recruiting, retrying and escalating only matter where the team fails. So every test runs on the
**failure set** (DeepSeek's 19 failures, plus its 6 false passes) and a **guard set** (20 random
problems DeepSeek solves, to measure harm and false escalations), not on all 118. That is ~45
problems instead of 118 per arm, and ~19 where the outcome can move. Confirmation on held-out
problems happens only for what wins: LiveCodeBench-Plus originals (25) and our 73-problem set.

## Reference point first (needed for every bar)
**R0. Opus 5.5 single call on the 118** (effort high). ~$7-9. Gives the frontier line on this set and
how many of the 17 nobody-solved problems a frontier model gets.

## Tests, in priority order

### T1. Correlated noise: recruit by rescue rate (Averbeck et al.: information is limited by correlated errors)
- Candidates, one call each on the failure set only: Qwen3.8-27B (xhigh; data partly in), Mistral
  Large 4, Mistral Small 4, MiniMax M3.1-Flash, Kimi K2.7-Code (a code specialist), DeepSeek V4-Pro,
  GLM-5.3 (full).
- Measure the rescue rate on the 19 failures, plus cost and time; false-pass rate on the guard set.
- Cost ~$5-15 (19-25 calls per candidate). Time: one or two 2-hour chunks, two candidates at a time.
- **Bar:** a recruit is worth it if it rescues ≥3 of 19 at ≤2× DeepSeek's cost per call.
  **Falsified** if no candidate rescues ≥3: then breadth is exhausted for this task and model tier.

### T2. Inhibition of return + adaptive exploration (basal ganglia; locus coeruleus adaptive gain)
On the failure set, three retry policies for DeepSeek's own failures, 2 samples each:
- (a) plain resample (same prompt, new sample);
- (b) inhibition of return: the retry is told the failed approach (one-line summary of the first
  program's method) and must use a different method family;
- (c) exploration: higher temperature plus the "different family" instruction.
- Cost ~$2-4. **Bar:** (b) or (c) rescues ≥2 more than (a). **Falsified** if (a) ≈ (b) ≈ (c): then a
  retry is just another sample and diversity must come from other models (T1).

### T3. Conflict-triggered control on false passes (conflict monitoring)
False passes (6 for DeepSeek) are invisible to the samples. Test whether disagreement catches them:
- For every problem, run all experts' programs on generated inputs in a Modal sandbox (no network;
  generators from the fresh-problem pipeline); conflict = DeepSeek's outputs differ from another
  sample-passing expert's.
- Measure: of DeepSeek's sample-passing programs, AUROC of "conflict" for hidden-test failure;
  how many false passes it flags; how many correct programs it wrongly flags.
- Then simulate "escalate on conflict" (to Opus 5.5 from R0, or to the best T1 recruit).
- Cost: Modal compute ~cents, generators ~$1. **Bar:** flags ≥3 of 6 false passes while flagging
  ≤10% of correct programs. **Falsified** if conflict is no better than chance on false passes.

### T4. Fast memory: case retrieval (complementary learning systems)
- For each failure-set problem, retrieve the 3 most similar *solved* problems from the other 99 (TF-IDF
  over statements) with DeepSeek's accepted programs, and put them in the prompt.
- Cost ~$1-2. **Bar:** rescues ≥3 of 19 without hurting the guard set. Leakage check: similar problems
  are different contests, so this is retrieval, not memorization of the answer.

### T5. Evidence threshold, revisited only if T3 works
The length rule failed; retest the sequential-stopping idea with the evidence that works (samples +
cross-expert agreement from T3). No new calls. **Bar:** beats the plain cascade on the frontier.

## Parked (bigger designs, after the gate exists)
Predictive coding (prediction-error routing between laminar layers), global-workspace broadcast,
slow consolidation (fine-tuning on prioritized replay), incubation as a batch-tier policy. Pruning is
already decided for this task: gpt-oss is dropped (adds nothing at 3x cost).

## Combined test (only after T1-T4)
Build the best team from the winners: DeepSeek first; on a check failure or conflict, the T2 retry
policy and/or the best T1 recruit; escalate to Opus only for the remainder. Pre-registered bar:
**≥ Opus 5.5's score on the 118 at ≤1/3 of its cost**, then confirmed once on held-out problems
(LiveCodeBench-Plus originals and our 73) with no further tuning.

## Budget and order
R0 ~$8 → T1 ~$10 → T2 ~$3 → T3 ~$1 + Modal → T4 ~$2 → T5 $0 → combined ~$5 → confirmation ~$10.
Total ~$40, well inside the remaining budget. Each step reports before the next starts.

## Results log

**R0 (2026-11-05): Opus 5.5 on the 118, effort high: 110/118, $0.053/problem, median 17 s, p90 55 s**
(regraded, no verdicts changed). Cascade DeepSeek > Opus (escalate when samples fail): 106 at $0.029
(55% of Opus's cost). Opus solves 11 of DeepSeek's 19 failures; 4 of those 11 are DeepSeek false passes
(passed samples, failed hidden tests), which a samples-only cascade never escalates. So catching false
passes (T3) is worth exactly the cascade's gap to Opus. Against the combined bar (Opus's score at ≤1/3 of
its cost, i.e. 110 at ≤ $0.0176): even a perfect T3 lands near 55-60% of the cost; the cost half needs
cheap rescues (T1/T2) to replace Opus calls. Bar unchanged.

**T3 (2026-11-05): conflict-triggered control on false passes: fails the bar.** 105 DeepSeek
sample-passing programs (6 false passes); every expert's program run on 3,147 Opus-written generated
inputs in an isolated Modal container. Disagreement with GLM: catches 2/6 false passes, flags 11% of
correct programs (AUROC 0.61); GLM + gpt-oss: 3/6 at 14% (0.67); Opus: 5/6 at 17% (0.79). Bar was ≥3/6
at ≤10%. As a cascade trigger, conflict with GLM reaches 107/118 (vs 106 samples-only) at $0.047/problem
(89% of Opus) because the checker's call is paid on every problem. Causes: cheap checkers share the
mistake (correlated errors); generated inputs that miss constraints make correct programs disagree
(no input validator yet). Best practical policy unchanged: DeepSeek, then Opus on sample failure, 106
at 55% of Opus's cost. Scripts: `bench/teamsolve/route/modal_t3.py`, `t3_score.py`.

**T1 (2026-11-05): recruit by rescue rate: the rescue bar is met only above the cost bar; breadth is
exhausted for the cascade.** One call per candidate on DeepSeek's 19 failures + 20 guard problems
(regraded). Rescues of 19 (of the 13 sample-failing ones a cascade knows to retry / of the 6 false
passes), guard, $/call: Opus 5.5 11 (7/4), 20/20, $0.053; Kimi K2.7-Code 4 (3/1), 20/20, $0.062; Mistral
Large 4 5 (4/1), 17/20, $0.25; GLM-5.3 1, 14/20, $0.17; Mistral Small 4 1, 10/20, $0.021; DeepSeek V4-Pro
1 of 8 done. Bar (≥3/19 at ≤2× DeepSeek's $0.011): Kimi and Large 4 rescue enough but cost 5.6× and 23×.
Cascades on the 118: DeepSeek > Opus 106 at 55% of Opus's cost; DeepSeek > Kimi-Code > Opus **104** at
58%; DeepSeek > Large 4 > Opus 107 at 113%; DeepSeek > Kimi-Code 102 at 40%. **A middle rung can
hurt**: its own false passes stop escalation before the frontier model. A middle rung needs a check
stronger than the samples. Qwen and MiniMax were not run (2-hour limit).
