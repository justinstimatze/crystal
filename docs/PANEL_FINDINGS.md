# Adversarial pressure-test panel — what survived, what fell (2026-05-29)

Four skeptics, each told to **refute, not confirm**, attacked the crystal thesis. Every surviving
number was re-verified against raw before it stayed a claim (house rule). Three of the four headline
refutations were re-verified by hand in this session, not just agent-claimed. This doc records the
verdicts and the casualties; the source/docs were corrected where a claim was overstated.

## The scoreboard

| Claim under attack | Verdict | Corrected form / fix |
|---|---|---|
| "det tier ~90,000× vs Haiku" | **REFUTED for the live hook** | in-process (serve) ~90,000×; the live **hook** pays process-startup, ~50–110× over a 640ms Haiku call |
| "the loop closes live" | **REFUTED → now FIXED** | was: hook demotes+flags, `author` re-authors, no code connects them. **Wired shut in `hook-loop`** (`HOOK_LOOP_FINDINGS.md`): demote→re-author→gate→swap→re-promote→resume, autonomous across 24 real processes. Mechanically autonomous; still oracle-dependent for the new class's labels (A5). |
| "g=0.77 deterministic coverage" | **WEAKENED → in-sample only** | g=0.77 on this user's Go/`gh` corpus; **untested** off-stack; a plausible data-science user → **g=0.00** |
| "latency breakeven = 43 hits" | **WEAKENED** | unstable point estimate; re-runs against the same cache give **36–37**; author latency is one draw from a 19.7–26.9s spread |
| live M-in-W demotion is robust | **REFUTED → two of three FIXED** | 2-in-5 interleave evaded **forever** → **fixed** (cumulative rate gate); terminal demotion was a **DoS** → **fixed** (re-promote path); confidently-wrong is **still invisible** (no live oracle — that's A5). |
| token breakeven ~2,944 (~70× slower) | **SURVIVES** | reproduced 2,825–2,915; order of magnitude holds |
| typed comparators > string equality | **SURVIVES** | outcome-class gate catches error↔success flips a `==` misses (but unused by triage/hook path) |
| author gate is load-bearing (0.01 reject) | **SURVIVES, already-caveated** | the gate holds a fixed bar; *but* its reference IS `detClassify` — fidelity-to-case-statement, not ground truth (AUTHOR_FINDINGS already says so). `hook-loop` re-confirmed it: a docker-only re-author was rejected at 4/8 before a full one passed at 8/8. |

## The eighth manufactured-confidence catch (mine)

The "~90,000× / ~7µs" determinism headline is the **in-process** function timing — correct for `serve`
(a batch that replays a cached call in-process). But `crystal hook` runs as **one fresh OS process per
PreToolUse event**. Measured here, 100 real forks: **~5.9ms/call** (the agent measured ~12.8ms on its
box) — entirely Go process-startup, which no coverage `g` removes. So the **deployed** deterministic
tier is **~50–110× faster than a 640ms Haiku call, not 90,000×**. I wrote the conflation in
HOOK_FINDINGS; this catch is self-inflicted. The µs number is real *for the in-process regime* and
stays in SERVE_FINDINGS scoped to it; the live-hook docs now carry the ~ms process floor.

> **Update (same session): this casualty is FIXED.** The seam was wired shut in `crystal hook-loop` —
> see `HOOK_LOOP_FINDINGS.md`. The section below is the original finding that motivated the fix.

## The load-bearing casualty: the live loop does not close (now wired shut)

Grep-verified: `authorRules` is called only by `author` / `author_drift` / `amortize`. **Nothing in
`hook.go` calls it.** `Demoted = false` appears **nowhere** — demotion is terminal; the only recovery
is deleting the `--state` file by hand. On live drift the hook writes the *string* "flagged for
re-authoring" into `additionalContext` and stops serving forever; no code reads that flag. The
"detect→re-author loop, now triggered live" was prose one wire ahead of the implementation. crystal is
a **self-demoting case statement with a manually-triggered self-author escape hatch** — a real
integration of demote-on-drift + gated self-authoring, but the *autonomous* framing is overstated.
Closing it for real (wire hook demotion → re-author → redeploy) is now the sharpest open build, ahead
of A5.

## Two new M-in-W evasions (extend `consecutive-divergence-demotion-is-evadable`)

> **Update (same session): evasions 1 and 2 are FIXED** — the cumulative rate gate catches the
> interleave, the re-promote path fixes the terminal DoS (`HOOK_LOOP_FINDINGS.md`, both unit-tested).
> Evasion 3 (confidently-wrong is invisible) stands — it needs a live oracle (ROADMAP A5).


1. **Sustained 2-in-5 interleave evades forever.** `covered, drift, covered, drift, …` holds
   steady-state at 2 uncovered per 5-window and **never** reaches M=3, while the table is wrong/silent
   on 40% of commands indefinitely. Real drift (new tool used *alongside* old ones) naturally takes
   this shape — so the gate misses the *normal* form of drift and only catches the artificial
   uninterrupted burst the test/demo is built around. The sliding window leaks in the **same
   direction** the consecutive-K rule did.
2. **Terminal demotion is a self-inflicted DoS.** One benign one-off `docker`/`kubectl`/`terraform`
   session demotes the tier permanently; every subsequent coverable `git`/`go` command (the agent
   showed 1000 of them) is deferred to the model though it could be served at 0 calls. No re-promote
   path exists. Under-fires on real interleaved drift, over-fires irreversibly on noise.
3. **Confidently-wrong is invisible.** The live signal is *coverage* (`cat != ""`), never
   correctness — so a covered-but-miscategorized command counts as "served" forever and injects the
   wrong category into the model's context with zero demotion. Live recall on wrong-answers is exactly
   zero by construction; only the batch `author` gate (with a reference) catches it.

## What actually survived (the honest core)

Strip the overstatements and crystal's defensible identity is narrow but real: **a self-demoting
case statement that knows when it has gone stale and says so (drift-demotion across real process
boundaries), with an expensive tier that can regenerate the rule table on demand against a
deterministic gate.** Neither a static 20-line bash `case` (can't tell it's stale) nor "just wait for
cheaper models" (no determinism, no local sovereignty, no zero-token verifiable substrate) occupies
that exact spot. The token-cost axis is conceded-collapsing; latency/determinism/sovereignty survive
the trend but are generic systems properties, not crystal inventions. The novelty is the *integration*
— which is exactly what the thesis already claims, **except** the "loop closes live" seam, which the
source contradicts.

## Fixes applied this session

**Doc/claim retractions:**
- `cmd/hook.go` demo epilogue + `HOOK_FINDINGS.md`: retract "the loop closes live"; state demote-and-flag
  honestly; add the ~ms process-startup floor and scope the µs/90,000× to the in-process `serve` regime.
- `HOOK_FINDINGS.md` host-capability section: split *binary* portability (true, zero deps) from
  *coverage* portability (host-specific, g→0.00 on a foreign stack, untested).
- `SLICE_FINDINGS.md` / `ROADMAP.md`: scope g=0.77 as in-sample on this user's corpus; generalization untested.

**Code fixes (the seam wired shut) — `cmd/hook.go`, `cmd/hook_loop.go`, `HOOK_LOOP_FINDINGS.md`:**
- Hook serves from a swappable rule-table artifact (`--rules`), not only the compiled baseline.
- `repromote()` added — demotion is no longer terminal (fixes the DoS).
- Cumulative rate gate added alongside the burst gate (catches the 2-in-5 interleave).
- Demoted tier keeps observing uncovered commands so the re-author sees the full drifted class.
- `crystal hook-loop` orchestrates the closed loop end-to-end across real processes (verified: 8/8 recover).
- 5 unit tests, including both panel evasions as regression tests.
