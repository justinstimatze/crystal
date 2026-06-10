# Sweep findings — crystal turned on its own substrate (2026-06-07)

The seed idea, stated in conversation and now run against real data: **a recurring instruction is a
crystallizable chore, and the chore being migrated is *remembering itself*.** Carrying a standing
rule in memory and re-applying it every turn is a frontier-tier operation — lossy, context-costly,
forgettable. A deterministic artifact (a hook, a `make` target, a git config) is the cheap tier:
reliable, zero context, fires unconditionally. So promoting a rule from *memory* to an *artifact* is
shift-left applied to remembering — and it's **auto-chunking**: bind an N-step recurring procedure
into one named unit so working memory holds one token (or zero), not N.

The recursion the user named: a memory rule is the first-order "don't make me remember"; it still
fails because *applying* it is a second-order act of remembering. The fix collapses the recursion —
move the constraint out of memory into the environment, where the wrong path is simply unavailable.
"Not having to remember to not have to remember."

## The sharp signal: re-encoded-despite-a-rule (deterministic, not transcript-mined)

The original idea was "promote a rule that got *violated* despite being remembered" — but detecting
violation needs a fuzzy temporal transcript scan. The sweep surfaced a **stronger, deterministic**
proxy: the *same rule independently written into N different projects' memories*. Each project
re-discovered and re-wrote the identical rule because the prior encoding didn't generalize. That is
recall provably failing to chunk — countable, verifiable, no model call.

Verified counts (`rg -l` across all `~/.claude/projects/*/memory` + every `CLAUDE.md` under
`~/Documents`; numbers re-counted from raw):

| recurring rule | projects that independently re-encoded it | mechanizable? |
|---|---|---|
| ban `git add -A` / `git add .` (stage explicit paths) | **4** — a private project, calque, lucida, plancheck | yes — PreToolUse deny |
| `main` not `master` as default branch | **3** — two private projects, plancheck (+ global `CLAUDE.md`) | yes — one global git config |
| never write secrets to files / source another project's `.env` | **3** — be-my-geminis, cupel, slimemold | yes — secret-write linter |
| estimate cost / confirm before paid or batch ops | **3–4** — cupel, lucida, a private project (+ root) | partial — hook on batch launch |

Corpus size: **156** `type: feedback` memory atoms and **758** rule-candidate lines across 18
`CLAUDE.md` files. The four above are the cleanest multi-project recurrences; the long tail is
mostly project-semantic (taste, citation philosophy, design grammar) — correctly *not* promotable.

## The existence proof already ships: weir

`weir` is a Claude Code **PreToolUse hook** whose `which`-vs-`command-v` rule emits a **deny** — a
recurring correction promoted to a deterministic blocking artifact. Its suggest-arm also nudges
(`grep | head` → `rg -m N`) and it fired on a command *in this very session*. So the artifact form
of the entire promote-set below **already exists**. The promote-set is not a system to build — it's
**new rules of weir's exact shape** that haven't been written yet.

## The promote-set (mechanizable → still recall-only)

Each has a deterministic oracle, so a hook can tell whether it was obeyed. Each buys, on promotion:
reliability (can't be forgotten), context reclaimed every future turn (the rule leaves the prompt),
and determinism (fires unconditionally).

1. **`git add -A` ban** — the flagship (4× re-encoded). A PreToolUse `deny` on `git add -A|.|--all`.
   weir-shaped; weir could host it directly.
2. **`main` not `master`** — `git config --global init.defaultBranch main` *is* the artifact; it
   retires the rule globally with one line. Cheaper than a hook.
3. **Co-Authored-By commit trailer** (global `CLAUDE.md`) — a `prepare-commit-msg` hook or commit
   template; the trailer stops being something to remember per commit.
4. **`gh repo create` → private by default** (global) — a wrapper / PreToolUse rule that injects
   `--private` or denies a public create without explicit override.
5. **Secrets-to-files refusal** (3× re-encoded) — a write-time linter (gitleaks-style patterns) of
   weir's shape; the redaction discipline already lives in `internal/redact`.
6. **End-of-turn `/schedule` offer ban** (a project-local rule + global) — a Stop-hook output linter, exactly
   the `which`-lint pattern applied to *my* output instead of a Bash command.

## The self-illustrating finding

The user has written **the crystal thesis itself** as a standing memory rule in **≥4 projects**:
- [hindcast] "does not want to remember to run diagnostic commands… anything that benefits from
  regular running" should fire on its own;
- [memory] "wants … tools to fire automatically via hooks/triggers, not require manual CLI";
- [memory] "feels friction having to volunteer everything; wants Claude to detect under-specified
  requests";
- [plancheck] "wants continuous work without being asked to stop or regroup."

A rule that says *stop making me remember* that had to be re-remembered and re-written per project is
the cleanest possible proof of the thesis — and the cleanest refutation of "just add a memory rule."
The memory rule existing and still not generalizing is not a gap in the memory system; it's
structural evidence that recall-and-hope tops out, and the only move past it is to mint the artifact.
Crystal is the artifact that retires this whole cluster at once.

## Honest scope — what this sweep is and isn't

- **Is:** a deterministic inventory + partition + verified cross-project recurrence counts. No model
  calls, no transcript reads into context (memory-footprint discipline honored).
- **Isn't:** transcript-mined violation evidence. "Re-encoded across N projects" is a *proxy* for
  recall failure, not a per-incident violation log. The proxy is stronger for being deterministic,
  but it does not prove any single later session *broke* a rule — only that the rule didn't chunk.
- **Flagship now BUILT:** `crystal guard` (`cmd/guard.go`) — a real PreToolUse hook that denies
  `git add -A | . | --all` with a stage-explicit-paths reason, verified end-to-end over the real
  stdin contract. The remaining promote-set (`main` via git config, Co-Authored-By trailer,
  `gh repo create` private-default, secrets linter, `/schedule`-offer Stop-linter) is still recall-only.

## Context-reclaim, MEASURED — and a self-tempering (2026-06-07)

The session's "context-budget is the cost axis that doesn't collapse" framing was an unverified
deduction (a reasoning-topology hook flagged it as load-bearing-and-unanchored across three turns).
Measured against raw bytes:

- Global `CLAUDE.md` = 28,539 chars ≈ **7,134 tok injected EVERY turn, every project** (the denominator).
- Clean always-injected *mechanizable* reclaim — the GitHub section (private-default + main-not-master,
  186 chars) + the `/schedule`-offer bullet (251 chars) = 437 chars ≈ **109 tok/turn = 1.5%** of the file.
- Measurement-honesty catch: per-project memory atoms that *match* a promote-set rule are whole
  190-line files (plancheck's is 8,033 chars), NOT the rule — the rule line itself is ~20 tok. Counting
  files as rule-cost would have inflated this ~25×; the rule text is tiny everywhere.

**Conclusion — tempering my own earlier claim:** context-reclaim is real and permanent but **small and
bounded** (~1.5% now). It does not collapse the way per-call tokens do, but it is a *minor absolute
quantity*, because (a) most standing guidance is irreducibly semantic and un-mechanizable, and (b)
you would never hand-carry thousands of mechanical rules in always-injected context, so the ceiling is
the mechanizable fraction of what's already there. **Latency / determinism / reliability remain the
PRIMARY value of shift-left; context-reclaim is a minor secondary, not a headline.** The slimemold
flag was correct; this is the kind of over-elevation the verify-against-raw rule exists to catch.

## How crystal tracks what it crystallized (the self-monitoring design)

Not a central babysitter — **each crystallized chore is its own sub-hybrid-loop, responsible for its
own QA**, and the *installed hooks are the registry* (the same "environment is the source of truth,
not a memory of it" principle the sweep itself relies on). A central manifest, if one exists, must be
**derived/reconstructable from the installed hooks**, never hand-maintained — otherwise the registry
drifts and you're back to remembering.

The drift signal differs by artifact kind, and the flagship makes the second kind concrete:
- **Classifier-type** (the `hook` categorizer): drift = coverage-collapse / wrong-answer; the M-in-W
  windowed demote. Already built (`hook-loop`).
- **Constraint-type** (`guard`): produces no answers to verify, so its QA signal is **override
  frequency.** `guard` counts denied-vs-bypassed in its `--state` file and flags `NeedsRevision` when
  a sustained bypass rate crosses the gate — "you keep overriding me, I'm probably wrong." That
  override gate is the constraint analog of the categorizer's coverage-collapse demote, and it's what
  makes the rule a self-monitoring loop instead of a dead deny.
