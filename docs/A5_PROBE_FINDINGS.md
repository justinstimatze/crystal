# A5 probe — the local cheap tier (`crystal local-probe`)

## ⇒ UPDATE 2026-06-07: two-model agreement oracle VALIDATED at N=250 (7× the N=37)

The agreement result below (N=37) was directional. Re-ran it on the live `--home` corpus — **250
unique commands sampled (deterministic stride) from 45,048 deduped reference-covered commands** —
with `local-probe --model qwen3:8b --model-2 qwen3.6:35b`. The N=37 shape **holds**:

| metric | N=37 | **N=250** |
|---|---|---|
| coverage (agree rate) | 0.76 | **0.74** (186/250) |
| accuracy ON agree | 0.86 | **0.87** (161/186) |
| accuracy ON disagree (8B / 35B) | 0.11 / 0.44 | **0.19 / 0.47** (12/64, 30/64) |
| Haiku baseline | 0.78 | **0.77** (193/250) |
| 8B / 35B solo | 0.68 / 0.76 | **0.69 / 0.76** |

**The claim survives scaling:** on the 74% where 8B and 35B agree, the agreed label is **0.87**
accurate — above **both** solo models (0.69, 0.76) — and the 26% they disagree on is exactly where
the errors live (0.19 / 0.47). Agreement is a real abstention oracle that concentrates correctness,
not a cherry-picked subset. This is the all-local trust signal at scale → it closes `hook-loop`'s
no-cloud-oracle gap (next step 1b: feed it to the re-author behind the deterministic gate).

**⇒ CLEAN-RERUN DONE (the loophole closed) — and it REPLICATED.** Cleared the qwen3 cache and re-ran
from scratch under one policy (300s timeout, no concurrent box polling). The clean draw: coverage
**0.80** (199/250), accuracy-on-agree **0.85** (170/199), concentration holds (on-disagree 8B 0.20 /
35B 0.43), Haiku 0.78, 8B solo 0.72, 35B solo 0.77. It does NOT match the resumed run digit-for-digit
(coverage 0.74→0.80, on-agree 0.87→0.85) — **because the live `--home` corpus GREW between runs**
(45,048→45,169 covered, 373→374 files: this very session logs commands to `~/.claude/projects`), so
the deterministic stride drew a *different* 250. That's not a bug — it's the honest truth that a
strided sample over a *growing* corpus isn't time-reproducible. The upshot is stronger than a single
run: **two independent 250-draws from ~45k both show the same shape** (coverage ~0.74–0.80, on-agree
~0.85–0.87, concentration every time). The oracle finding replicates across resampling.

- **Clean latency now citable (8B):** p50 **225ms** / p99 **327ms** — fully resident, tight tail; ~2.5×
  faster than Haiku (p50 553ms). **35B clean latency is NOT surfaced** — the probe only aggregates
  `--model` (8B) latency, not `--model-2`; surfacing model-2 latency is a small probe fix (TODO) before
  the 35B side of the latency story is quotable.
- **For byte-reproducibility**, freeze a corpus snapshot (the live `--home` set moves under you); the
  small `testdata/corpus` is already frozen and is where N=37 reproduces exactly.
- The `verdict` line still flags the *solo* 8B as too weak (0.72) — correct; the *agreement* oracle
  (0.85 on 80%) is the proposal, not the solo model.

## ⇒ UPDATE 2026-06-07: the negative is OVERTURNED on accuracy (GPU + a capable model)

The 2026-05-29 negative below was a **toy-model artifact**, not a local-tier verdict. Re-run against a
real model on real hardware — `qwen3.6:35b` (a 36B MoE) on the user's RTX 3080 box over the LAN
(`OLLAMA_HOST=http://192.168.4.114:11434`, zero code change — `internal/local` already reads it):

| tier | accuracy vs det | p50 | p99 |
|---|---|---|---|
| det (`detClassify`) | 1.00 (reference) | ~µs | ~µs |
| Haiku (cloud cheap) | 0.78 (29/37) | 627ms | 1102ms |
| **qwen3.6:35b (LOCAL, 3080)** | **0.76 (28/37)** | **3298ms** | **22523ms** |

- **Accuracy blocker GONE.** 0.76 **ties Haiku** (0.78, one row apart) — vs qwen2:1.5b's 0.46. A
  capable local model matches cloud-cheap accuracy on this chore. (Recounted from raw: 37 rows,
  haiku✓=29, local✓=28.)
- **Pre-flight bug caught (would have faked a 0.00).** `qwen3.x` is a *thinking* model; under the
  16-token cap it spent the whole budget on hidden `thinking` and returned `response:""` — every
  command a parse-fail. `internal/local`'s comment *claimed* "thinking-free" but never set the flag;
  fixed (`Think:false` on the generate request). Thinking off → answers directly, warm short call 0.86s.
- **Latency is a model-VRAM-fit problem, not GPU absence.** 24GB model on a 10GB card → heavy CPU
  offload → p50 3.3s. The old verdict hardcoded "CPU, no GPU"; fixed to not assert the remote host's
  hardware. **Next:** a 7–8B at Q4 fits ~5–6GB *fully resident* → expected Haiku-level accuracy AND
  sub-second latency.
- **Live-oracle, reframed.** Both Haiku and local sit ~0.77 *against det* — the ceiling is det's
  debatable edge conventions (every `gh` subcommand labeled `network`; the model says `other`/`git`,
  arguably more right), not model weakness. 0.90-vs-det is the wrong bar. A **confirm step** (local
  proposes, a slower model ratifies) is the viable oracle path — and it can be **all-local on one box**
  (GPU fast proposer + CPU careful ratifier/re-authorer, `num_gpu:0` to pin the second to CPU).

### The frontier, completed — and a two-model agreement result (same day)

Pulled `qwen3:8b` *remotely* (ollama `/api/pull` — the whole flow is HTTP: pull, load, unload,
`/api/ps`; the GPU box is never touched by hand) and measured it. It fits **fully VRAM-resident**
(`/api/ps`: 5.5GB/5.5GB, 0 in RAM). Three measured points now:

| tier | acc vs det | p50 | p99 | VRAM |
|---|---|---|---|---|
| Haiku (cloud) | 0.78 | 627ms | 1102ms | — |
| qwen3.6:35b (local, spills) | 0.76 | 3298ms | 22523ms | 30% resident |
| qwen3:8b (local, resident) | 0.68 | **219ms** | 424ms | 100% ✓ |

A clean accuracy/latency frontier: the 8B is **3× faster than Haiku** but 0.68 (too weak solo); the
35B ties Haiku but is slow (spills to CPU). This is the proposer/ratifier split in real numbers.

**Two-model agreement (cross-tab of the two cached probes, N=37):**
- 8B and 35B **agree on 76%** of commands; when they agree, accuracy is **0.86** — above either alone
  (0.68 / 0.76) and above Haiku (0.78). Agreement concentrates correctness.
- Disagreements (24%) flag the hard/ambiguous cases (35B right 4, 8B right 1, neither 4 — the
  det-edge `gh`→`network` family).

**Self-caught invalid number (the discipline working):** an initial "blended latency ≈ 968ms" for a
confirm-step was **WRONG and is retracted**. The escalation trigger used was *"the two models
disagree,"* which requires running BOTH models on every command — so you never skip the 35B; real
latency is ≥3.3s, no speedup. The figure claimed a saving the method cannot deliver.

**What the agreement result actually buys (better than latency): an all-local TRUST signal — the
live-oracle gap, closed locally.** The open problem from [[hook-loop-closes-the-seam]] was trusting a
local label for a new class with no oracle. Answer: run two independent local models; **trust the
label on agreement (0.86, 76% of commands), abstain/escalate on disagreement.** No cloud, no human, no
confidence plumbing — agreement IS the signal. *For a real latency win* you need a proposer-ONLY
trigger (8B logprob/entropy → escalate only the uncertain ~24% to the 35B); that is unmeasured, the
honest next experiment. Caveat: N=37, directional not robust.

*Status: A5 accuracy de-risked; the two-model agreement gives an all-local label-trust signal; a
proposer-confidence latency experiment is the next step. Original (toy-model) write-up kept below.*

---

# A5 probe — the local cheap tier (2026-05-29, superseded above)

A5 is the sovereignty rung: swap the residual's cheap tier from cloud Haiku to a LOCAL model on owned
hardware. Before building the full slice, one probe — the way `probe` de-risked the cloud tier — asks
the question A5 turns on: **can a local small model do the categorize chore well enough, and fast
enough, to be the cheap tier — and, by the same accuracy number, to be the live ORACLE the `hook-loop`
re-author still lacks?**

Measured on the COVERED fraction (37 commands the deterministic rules cover, so `detClassify` is
ground truth — the only place with trustworthy labels). All numbers verified against the raw
`--verbose` rows by independent recount (37 rows, haiku✓=28, local✓=17).

## Result (this host: CPU, NO GPU — the brief's RTX 3080 is not reachable here)

| tier | accuracy vs det reference | latency p50 | latency p99 |
|---|---|---|---|
| deterministic (`detClassify`) | **37/37 = 1.00** (IS the reference — *after* the bug fix below) | ~µs | ~µs |
| cloud cheap (Haiku) | **29/37 = 0.78** (was 0.76 against the buggy reference) | 627ms | 1102ms |
| **LOCAL (`qwen2:1.5b`)** | **17/37 = 0.46** | **2470ms** | **16299ms** (one outlier 25s) |

## Verdict: the naive local tier does NOT pay on this hardware — a clean negative

- **Not viable as a cheap tier.** `qwen2:1.5b` loses on BOTH axes: accuracy 0.46 (worse than a coin
  flip across 8 classes is ~0.125, so it's learning *something*, but it's far below Haiku's 0.76 and
  the deterministic 1.00) AND latency (p50 2.5s vs Haiku 0.6s — ~4× SLOWER, because it runs on CPU
  with no GPU). The shift-left value prop (cheaper/faster behind a gate) inverts here: local is both
  worse and slower.
- **Too weak as a live oracle (0.46).** This was the real prize — a local label source would let the
  `hook-loop` re-author discover a new class's ground truth without the provided `containerRef` oracle
  (the gap [[hook-loop-closes-the-seam]] left open). At 0.46 it would feed the gate bad training
  labels half the time; the gate would (correctly) reject the re-authored table, or worse, promote a
  table trained on noise. **The no-live-oracle gap stays open.**

## The bonus finding — AUDITED (the ninth manufactured-confidence catch, self-caught)

The first draft of this doc claimed Haiku's disagreements with det were "genuine compound-command
ambiguity, not pure error." That was a fluent *vibes* claim — asserted, not inspected. The slimemold
reasoning hook flagged it as load-bearing-vibes, so I audited all 9 Haiku misses against the raw rows.
The audit overturned part of my own claim:

- **8 of 9 Haiku misses are det-correct** — `mkdir`→file-edit, `chmod`→file-edit, bare `cd`→nav,
  and the genuine compound/`cd`-reduce cases (`cd X && <action>` where det's reduce wins and the model
  is fooled by the leading `cd`). So the ambiguity story holds for most, but as *model error against a
  correct rule*, not symmetric ambiguity.
- **1 of 9 was a `detClassify` BUG, not ambiguity.** `until curl … | grep -q 200; do …; done; … && tail …`
  was labeled `search/inspect` because det inspects only each segment's LEADING token, and the `until`
  keyword masked the `curl` — so det landed on the `grep`/`tail` segments. The right label is `network`
  (a URL poll), and Haiku had said exactly that — it was marked *wrong against a buggy reference*.

**So "det = 1.00 ground truth" was contaminated by 1/37 (2.7%).** Fixed: `segClassify` now strips
leading shell control-flow / timing wrappers (`until`/`while`/`time`) that hide the action word — the
same bug family as the original leading-`cd` compound fix, found the same way (auditing real data),
excluding sudo/env/xargs (they interpose flag-args). Regression test in `cmd/triage_test.go`. After
the fix the reference is corrected and **Haiku rises 0.76 → 0.78** (its `until curl`→network call now
agrees with det); local stays 0.46.

The thesis direction survives the correction, now on a *clean* reference: **on its covered fraction the
deterministic tier is the MOST ACCURATE tier, not just the fastest** (1.00 > 0.78 > 0.46). Cheap tiers
earn their place only on the *residual* the rules can't cover; inside coverage, the corrected rule
table dominates. "Don't reach for a model where a rule is exact" — but *do* audit the rule against raw,
because the reference itself can carry a bug that silently mismeasures every tier compared to it.

## What this de-risks, and the path to a viable A5

The probe did its job: it showed the **naive** local tier (a 1.5B model, CPU, off-the-shelf, no
fine-tuning) is not viable — *before* the cost of building the full slice around it. A viable A5 needs
at least one of:
- **A GPU** (the brief's RTX 3080, not on this host) — would cut the 2.5s p50 toward Haiku's range or
  below; latency is a hardware problem, solvable.
- **A stronger local model** (7B–8B class) — likely closes much of the 0.46→0.76 accuracy gap;
  feasible on a 10GB GPU.
- **+LoRA fine-tuning on THIS chore** — the brief's actual bet; a tuned small model could match the
  deterministic reference on covered commands and propose trustworthy labels on the residual.
- **A confirm step** — treat a weak local oracle's labels as *proposals* a human or a cloud call
  ratifies before they train the gate (cheaper than full cloud, safer than trusting 0.46).

Until then, the cost gradient's far end is established as *aspirational on this hardware*: local
inference exists and is wired (`internal/local`, ollama), but a small CPU model is the wrong end of
the accuracy/latency frontier for this chore. The honest status: **A5 is plumbed and measured, not
yet paying** — the sovereignty pitch needs the GPU + a stronger/tuned model the brief assumed.
