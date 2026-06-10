# Closing the loop — `crystal hook-loop` (2026-05-29)

The 2026-05-29 panel's load-bearing casualty was that **the live loop did not close**: `hook` demoted
and wrote a re-author flag nothing read; `author` re-authored in a separate process; no code connected
them, and demotion was terminal (a one-off `docker` session permanently killed the tier). "The loop
closes live" was retracted. `hook-loop` wires the seam shut and the two M-in-W evasions are fixed.

## What was built

**1. The hook serves from a swappable artifact, not the compiled baseline.** `crystal hook --rules
<path>` loads a tier-authored `ruleTable` JSON and classifies from it (falling back to `detClassify`
if absent/corrupt — fail-open). This indirection is what lets a re-author change *live* serving
behavior: the loop swaps the file, the next hook process serves the new table.

**2. Re-promotion exists (the terminal-DoS fix).** `repromote()` is the inverse the old hook lacked:
it clears `Demoted`, resets the drift windows, and counts the recovery. Demotion is no longer a
one-way door — verified by `TestRepromoteRecoversFromTerminalDemotion`.

**3. A cumulative drift gate (the interleave-evasion fix).** Alongside the fast burst gate (M-in-W),
a slow gate demotes when the trailing long-window uncovered **rate** exceeds a threshold (default
0.35 over ≥12 of the last 20). The panel's 2-in-5 interleave (`C C C U U …`, 40% uncovered, never
3-in-5) evades the burst gate forever but trips the rate gate.
`TestSustainedInterleaveEvadesBurstButCaughtByCumulative` proves *both* directions: burst-only stays
blind across 40 commands; the cumulative gate catches it and tags the reason `sustained`. The
threshold must sit above the host's baseline residual rate (1−g) and below the drift rate — a
host-specific tuning the doc names rather than hides.

**4. A demoted tier keeps observing.** While demoted, the hook still records uncovered commands into
`recent_uncovered` so the pending re-author sees the *full* drifted class, not just the few examples
before demotion. This was load-bearing: the first end-to-end run re-authored on only the 3 docker
commands seen pre-demotion, produced a docker-only table, and the gate **correctly rejected it at
4/8** (podman/kubectl uncovered). Capturing post-demotion examples is what lets recovery actually
succeed — and the rejection first proved the gate is not a rubber stamp.

**5. `hook-loop` orchestrates the closed loop across REAL processes.** author v1 → write artifact →
drive the compiled `crystal hook --rules` over normal+drift (separate process per event) → demote
live → read the demotion + captured drift sample → re-author with the class in scope → gate on the
drift class → on pass, atomically swap the artifact and re-promote → re-drive the SAME commands.

## The end-to-end run (the closed loop, across 24 separate processes)

```
=== 2/3. drive `crystal hook --rules <artifact>` over 16 live events ===
  [ 0..7] serve-det    (8 normal commands, served from the AUTHORED artifact, 0 model calls)
  --- injected drift: 8 container commands the v1 rules never saw ---
  [ 8] defer→model  docker build -t app .
  [ 9] defer→model  docker run --rm app
  [10] DEMOTE!      docker ps -a            ← burst gate, 3rd uncovered in window
  [11..15] defer→model  (podman/kubectl/compose — still observed into recent_uncovered)

=== 4. re-author: DEMOTED (burst gate), captured 8 drifted commands; re-authoring ===
  re-gate on the drift class: 8/8 = 1.00 (gate 0.90)
  → PROMOTE: atomically swapped the artifact (15 rules) and RE-PROMOTED the tier.

=== 6. resume: re-drive the SAME container commands through the live hook ===
  [ 0..7] serve-det    docker build / run / ps / podman / kubectl ×3 / compose — all SERVED

=== outcome ===
  demoted live at stream index 10, re-authored (burst gate), re-promoted, and now serve 8/8 of the
  once-drifting container commands deterministically (0 model calls) — the loop CLOSED across
  24 separate hook processes, autonomously, with no human re-running `author`.
```

## What this proves — and the honest line it does NOT cross

**Proven:** the detect→re-author→gate→swap→re-promote→resume loop runs **autonomously across real
process boundaries**, with no human re-running `author`. Terminal demotion is fixed (the tier recovers
itself); the two M-in-W evasions the panel found are closed (interleave caught by the rate gate;
DoS fixed by re-promote). This is strictly more than the disconnected commands that shipped before —
the seam the panel exposed is wired shut.

**NOT crossed (the remaining gap, named not hidden):** the re-author's reference labels for the NEW
class come from `containerRef` — a *provided oracle*, the same fidelity-to-reference caveat `author`
already carries. The loop closes the **wiring** given a reference; **discovering** the new class's
ground truth with no oracle is the open problem. A live hook has no oracle (that is why its drift
signal is coverage-collapse, not wrong-answer), so genuinely autonomous re-authoring needs a label
source — a second model, a user confirmation, or a local judge. That is exactly the local-model rung
(ROADMAP A5). So: the loop is **mechanically autonomous, epistemically oracle-dependent.** Don't
overclaim it as the latter — that was the mistake this session's panel caught the first time.
