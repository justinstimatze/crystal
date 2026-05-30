# The live PreToolUse hook — `crystal hook` / `crystal hook-demo` (2026-05-29)

Every prior result (`triage`, `author`, `serve`, `amortize`) ran the shift-left stack as a **batch
over a corpus**. The honest gap was that nothing had ever served in a real loop: a fresh process per
tool use, no in-memory state to lean on, the actual Claude Code hook contract. `hook` closes it.

## What it is

`crystal hook` is a real Claude Code **PreToolUse** hook. It reads a tool event on stdin and writes
a hook decision on stdout, exit 0:

```
stdin:  {"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}
stdout: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow",
         "additionalContext":"[crystal] Bash command category (deterministic, 0 model calls): git. ..."}}
```

The chore is the same one `triage` ships — categorize the Bash command — but now answered **in place
of a frontier call, live**. On a covered command the hook injects the deterministic category as
`additionalContext` (a system reminder the model reads), so the frontier never classifies it: **0
model calls on the covered fraction**. On the residual it stays silent and defers to the model. It
**never denies** — it only answers.

## The live part: drift state across process boundaries

Each PreToolUse call is a separate process. The windowed M-in-W drift state exists only because it
round-trips through a small JSON state file — that disk round-trip **is** the "live" part: the
demotion accumulates across real process boundaries, not in one in-memory loop.

The drift signal is **deliberately different** from `author`'s. A live hook has **no oracle**, so it
cannot detect *wrong* answers (that needs a reference). What it *can* detect with no oracle is
**coverage collapse** — the rule table going silent on a sustained run of commands. Same windowed
M-in-W rule (`internal/drift`); the signal is "stopped being able to answer," not "answered wrong."

## The end-to-end run (`hook-demo`, driving the REAL binary over 24 events)

`hook-demo` is not an in-process simulation: it `exec`s the compiled `crystal hook` binary once per
command, piping a PreToolUse JSON event to its stdin and parsing its stdout — the drift window
surviving only via the on-disk state file. It streams 16 real corpus commands (the normal regime),
then injects the 8-command container class `author` uses as its known drift.

```
=== normal regime: 16 real commands (testdata/corpus) ===
  [ 0] serve-det    ls -la $HOME/Documents/antiphon/ && ls $HOME/.cl…
  ...
  [14] serve-det    cd prototypes/bid-heatmap-ui && rg -n '"version"…
  [15] defer→model  python3 -c "import google.genai; ...

=== injected drift: 8 container commands the rules never saw ===
  [16] DEMOTE!      docker build -t app .
  [17] defer→model  docker run --rm app
  ...

=== outcome ===
  DEMOTED live at stream index 16 (the 3-in-5 window collapsed on the container burst).
  served deterministically (0 model calls): 10  ·  deferred to model: 20
```

**What the run shows (corrected after the 2026-05-29 panel — see `PANEL_FINDINGS.md`):** 10 of 16
real commands served deterministically (0 model calls), the 6-command residual deferred silently
without ever false-demoting, and the injected container burst demoted the tier — after which the hook
injects nothing and writes a re-author *flag*. **The loop does NOT close live.** Grep-verified:
nothing in `hook.go` calls `authorRules`; `author` is a separate command a human runs; `Demoted`
is never unset (terminal demotion, recover by deleting `--state`). The flag string is cosmetic —
no code reads it. So this is **demote-and-flag**, not autonomous detect→re-author→redeploy. Wiring
that seam is the sharpest open build (ahead of A5).

**Deployed cost (the process-startup floor the µs figure omits):** each PreToolUse call is a fresh
OS process. Measured here, 100 real `crystal hook` forks: **~5.9ms/call** — Go process startup, which
no coverage `g` removes. Over a 640ms Haiku call the deployed speedup is **~50–110×, not the
~90,000× / ~7µs in-process figure** (that figure is real for `serve`, an in-process cache replay, and
stays scoped to it in `SERVE_FINDINGS.md`). The eighth manufactured-confidence catch was conflating
the in-process function timing with the out-of-process hook this very file ships.

## Honest finding: the coverage trigger conflates residual clusters with drift

The live demo demoted at the **first** container command (index 16), not the third — even though
M=3-in-5 nominally needs three uncovered commands. Verified against the raw stream: the trailing
normal window already held 2 residual commands (indices 13 and 15), so the first drift command was
the 3rd uncovered in the window. The windowed coverage trigger sees only **coverage**; it does not
distinguish "a tail cluster of normal residual" from "a domain shift." With g≈0.77 the baseline
uncovered rate (~0.23) sits well below the 0.6 the 3-in-5 trigger needs, so normal residual alone
does **not** demote (it didn't, across all 16) — but a residual cluster plus a little real drift tips
it one command sooner than the burst alone would. This is a real property to tune, not hide: it ties
the live trigger to the same M-in-W sensitivity question as
`consecutive-divergence-demotion-is-evadable` — widen W or raise M to separate drift from residual
noise, at the cost of slower demotion. The clean-prefix case (demote exactly on the 3rd uncovered) is
asserted in `cmd/hook_test.go`.

## Host-capability (the weir caveat, answered)

The roadmap flagged that the deterministic tier may lean on installed tools (weir's manifest). For
**this** rule table the *binary* dependency is **zero**: `detClassify` is pure Go string matching — it
shells out to nothing, so the hook runs on any host. The caveat is real but class-specific: a rule
table that delegated to `rg`/`fd` *would* carry the dependency and would need a capability probe +
fallback (weir's SessionStart manifest is the reuse).

**But binary-portability is not coverage-portability** (the panel's sharpest portability catch).
"Runs anywhere" ≠ "answers anywhere." `detClassify` is a hand-tuned `switch` over ~45 tokens specific
to *this* user's stack (`git`, `gh`, `go build`, `cargo`, the weir toolchain). g=0.77 is **in-sample**
— measured on the corpus the rules were tuned against; **no held-out corpus exists** (`--home` only
re-reads this same user's other machines). Run through the real `detClassify`, a plausible
data-science user's commands (`python3`, `jupyter`, `conda`, `docker`, `kubectl`, `terraform`, `dvc`,
`uv pip`, `awk`) score **g=0/30 = 0.00** (verified in-package this session). So the honest scope is:
the *binary* is host-portable; the *coverage* is host-specific and, off this stack, can be zero.
Documenting which kind of rule table you serve — and that g is in-sample — is the discipline.

## Verification (against raw, house rule)

The raw stdout was inspected per case, not inferred:
- covered (`git status`) → `permissionDecision:"allow"` + the category in `additionalContext`;
- residual (`weirdcmd --xyz`) → `allow`, **no** `additionalContext` (silent defer);
- non-Bash (`Read`) → `allow`, no context, and the state's `total` did **not** increment (correctly
  not our chore).
The state file after the run shows the window, served/deferred tallies, and `demoted` flag round-trip
intact across the separate invocations.

## Wiring it into Claude Code (the real artifact)

See `docs/hooks/settings.snippet.json`. Register `crystal hook` as a PreToolUse hook matching `Bash`;
it persists its drift window to `--state`. The hook is fail-open: if it cannot parse its input or load
its state it emits a plain `allow`, so it can never block the user's command.

## Bottom line

The shift-left stack now *serves* **live**, not just in a benchmark: a real PreToolUse hook answers a
recurring chore deterministically with 0 model calls on the covered fraction (at ~5.9ms process-fork
cost, ~50–110× over a model call), defers the residual, and **demotes** itself across real process
boundaries when a domain shift collapses its coverage. *But the loop does not close on its own* — the
hook demotes and flags; re-authoring is a separate manual `author` run (see `PANEL_FINDINGS.md`). Two
open builds, in order: (1) **wire the seam** — hook demotion → auto re-author → redeploy — and harden
M-in-W against the interleave/terminal-DoS evasions the panel found; (2) the local-model cheap tier
(ROADMAP A5), the sovereignty end of the gradient.
