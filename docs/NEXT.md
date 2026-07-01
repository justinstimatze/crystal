# Working priorities (rolling)

Status: working note. Reranked 2026-06-30 (second pass). Ordered by leverage
against the README vision, not by ease. Items move; this file is meant to be
rewritten.

## The open frontier (reranked, second pass)

The three that led the last rerank are all BUILT and committed (#1 `sweep
--emit-library`, #2 `internal/recipe` + `crystal recipe`, #3 `crystal
planshift`). They now drop below the fold as DONE. The live question is the
*tail* they exposed, reordered by leverage:

1. **Settle #3's plan-recovery question with a middle-band chore.** *(BUILT —
   `planshift --chore-dir testdata/planshift-chores`. Outcome: no middle band
   exists for this chore family, and the pursuit caught a 3rd instrument bug that
   had manufactured a *false* one.)* Authored four graduated synthetic enum
   commands (1→3 enums, 3→5 flags) as a dedicated chore corpus behind a new
   `--chore-dir` flag (kept out of the real CLI; the Go toolchain ignores
   `testdata/`). Real result after the fix below: **none 4/4, plan-ungated 4/4,
   plan-gated 4/4.** The weak executor reconstructs every complete-spec enum
   contract with or without a plan, so plan-gating has no downside to remove —
   still ABOVE THE FLOOR, no discrimination.

   The deeper, honest reasons it can't be settled *here*:
   - **Structural confound.** `planshift` hands every arm the same complete
     `choreSpec` (enum values included), so a plan can add emphasis but never
     *information*. For a mechanical complete-spec chore, plans are *correctly*
     inert — that is not a null, it is the right answer. The standing
     43%-vs-57% result was on a *semantic* chore (`categorize`), where the value
     the plan adds is exactly the information a spec can't pin down. A
     code-scaffold harness with a complete spec cannot represent that band.
   - **The 3rd instrument bug (the cautionary catch).** Before the fix, the run
     showed a *seductive false result*: gated 0/4, pClean anti-correlated with
     execution, reading "prescriptive plans mislead the weak model AND the gate
     picks the worst one." Inspecting the raw produced source (a new
     `--verbose` source dump on contract mismatch) showed the scaffolds were
     contract-*correct* in kong's grouped tag dialect
     (`kong:"enum='a,b',help='...'"`); `cmdspec` only read the discrete
     `enum:"..."` form, so it reported `enums=[]` — a false negative. The
     verifier — this project's "master variable" — was itself the bug, and a
     subtle parser gap fabricated a compelling finding. Fixed `cmdspec` to read
     both kong dialects (+ `TestParseKongGroup`, `TestGroupedDialectEnum`); this
     silently affected the `dogfood` harness too.

   Takeaway: the plan rung stays demoted for *mechanical* chores (plans are
   inert when the spec is complete — correct, not a failure); its recovery
   question lives on *semantic* chores and needs a harness whose spec is
   deliberately incomplete. And: always read the produced artifact before
   trusting a discriminating result — a narrow verifier invents them.
2. **Make the local-open rung real (fill the deferred transfer cell).** *(BUILT
   — `transfer --open-model`, served by `modal/openmodel_server.py`.)* The
   openness axis was asserted end-to-end but the transfer cell was deferred
   ("away from the 3080"). Filled it with a Modal-hosted vLLM open model instead
   of waking the house box (no VRAM-spill stall): an OpenAI-compatible endpoint
   the same client that backs the PublicAI tier speaks to (`publicai.NewAt`), so
   crystal's harness stays authoritative — Opus authors the recipes in Go,
   golden-match (gofmt-canonical) in Go, disk-cache in Go; Modal is only the
   open-model compute behind a URL.

   **Measured (Qwen2.5-32B-Instruct, chore = entity→Go struct, n=7):**
   none 43% → plan 57% → recipe 71% → pseudocode 71%. Two honest reads:
   - **The open ~32B TIES the closed cheap tier's best rung** (Haiku 71% at
     recipe/pseudocode). An open, self-hostable model reaches the paid-closed
     cheap tier once given the specific recipe — the A5 "qwen3.6:35b ties Haiku"
     claim, reproduced on the transfer chore. This is the cell that was most
     contestable; it now reads *tie*, not gap.
   - **The recipe ladder holds MORE cleanly on the open model.** Its transfer
     rises monotonically with rung specificity (43→57→71→71), where Haiku *dips*
     at the vague `plan` rung (57→43→71). The "right specificity, not more words"
     shape is crisper on the open tier here.

   Caveats, foregrounded: **n=7** — suggestive, not conclusive. A 7B smoke run
   was *flat* at 71% across all rungs (above this chore's floor), and its
   no-recipe score (71%) sitting above the 32B's (43%) is within 2-unit noise —
   do NOT read "7B > 32B". The measurement is disk-cached (gitignored), so it
   reruns free; the endpoint is a throwaway deploy (`modal app stop` after).
   Two Modal infra bugs caught and fixed en route: vLLM 0.6.6's ZMQ IPC frontend
   throws `ENOTSUP` under Modal (fixed with `--disable-frontend-multiprocessing`);
   and the model id read from a module global inside the remote `serve()` fell
   back to the 32B default and OOM'd a small GPU (fixed by baking `VLLM_MODEL`
   into the image env and reading it inside `serve`). Was old-#4.
3. **Wire `library` serve into a live hook.** Serve is demoed over a canned
   stream; make it fire on real PreToolUse events like `guard`/`dispatch` do.
   This is what makes "ambient, no asking" true at the serve layer, and the
   guard/dispatch pattern is already there to copy. Was old-#5.
4. **defn `test <name>` dispatch** — keep deferred (sibling deadline); the
   `--verifier test` raw-`go test` shim covers it for now. Was old-#6.
5. **slimemold as the knowledge-work gate signal** — defer until the code-side
   verifier swap is solid; it is the hard (semantic) end and should not be
   load-bearing yet. Was old-#7.

The reorder vs. last pass: the three BUILT items retire to the DONE archive
below; the trailing "author a middle-band chore" note (from old-#3's finding)
becomes the new #1 because it is the cheapest conversion of an asserted claim
into a measured one. #4→#2 and #5→#3 keep their relative order.

## DONE archive (last pass's top three)

1. **Author library entries from observed recurrence** — close the watch→author
   half of the serve tier. *(BUILT — `crystal sweep --emit-library`; see below.)*
   Was the top gap: the library was hand-seeded, which made the "ambient, no
   asking" claim hollow at the serve layer.
2. **Give the crystallized artifact a real schema.** *(BUILT — `internal/recipe`
   + `crystal recipe`; `docs/RECIPE_SCHEMA.md`.)* The rungs are now a typed,
   diffable, lint-able object (rung × executor × verifier × narrow-waist
   inputs/output + residual), with the rung↔executor coupling enforced and a
   degrees-of-freedom metric. Designed against Odendahl's generalization-shaping
   framing; cites his two essays. The same entity→struct task ships authored at
   four rungs to show one chore descending the ladder.
3. **Transfer harness on a real code-change chore, plancheck-gating the `plan`
   rung.** *(BUILT — `crystal planshift`. Result: INCONCLUSIVE, honestly.)* Chore
   = reconstruct a kong subcommand from its contract; plan rung gated by the real
   `plancheck` binary (author K plan styles → `forecast.pClean` − co-mod-gap
   penalty picks the best → abstain if below bar); verifier = the produced
   scaffold's enum/slice/flag contract via `cmdspec` (AST-only, no scratch
   builds). Three arms: none / plan-ungated / plan-gated.

   **Finding:** the harness, gate, and verifier all work, but the measurement
   doesn't discriminate. On big commands (7–9 flags) every arm fails (below the
   weak executor's floor, 0/3); on small slice-contracts (2–4 flags) every arm
   passes (above the floor — Haiku reconstructs them with *no* plan, 3/3/3). Plan-
   gating can only help in the MIDDLE BAND (fail without a good plan, pass with
   one), and neither chore family hits it. The band a kong scaffold needs is a
   *small enum contract* (the canonical dropped-tag from dogfood) — crystal's enum
   commands are all large, so this substrate can't produce it. `plancheck`'s
   `pClean` was stable (~0.64–0.70) and correctly zeroed the unparseable
   `test-first` plan, but did not predict exec success (everything passed). The
   harness self-diagnoses this (ABOVE-THE-FLOOR reading). Two real instrument bugs
   were found and fixed en route (double `package` clause; chat-style weak-model
   output needing first-code-block extraction), so the instrument is validated —
   it's the substrate that lacks a discriminating chore. Its trailing finding —
   author a middle-band chore (a small synthetic enum command) — is now the live
   **#1** at the top of this file.

## Pointers worth folding in (artifact-shape critique)

- **The artifact isn't structured enough to be the product.** crystal emits Go
  code and prints prose recipes, but the crystallized *thing* — recipe, library
  entry, verifier contract — should be a first-class declarative object you can
  pipe, filter, render, and diff. "Portable across model regimes" is only true if
  the portable unit has a schema. (This is #2.)
- **Emit structured rows, not tables.** `transfer`, `library`, `sediment` print
  human tables; make them emit structured records with the table as a *render* of
  that, so stages compose instead of being terminal.
- **Recipes are prompts; version them as templates.** The author-fn loop versions
  emitted *Go*; the emitted *recipes/prompts* are unversioned strings — they're
  the artifact that runs on the weak tier and deserve the same discipline.
- **The verifier-as-master-variable framing is the strongest part** — lean into
  it. The eval/feedback gate being the bottleneck (not the model tier) is the
  defensible core.
- **Don't let "one binary" hide a pipeline.** Each stage (discover / author /
  verify / serve) wants to read structured input and write structured output,
  assemblable — same insight as #2 from a different angle.

## What #1 looks like, built

`crystal sweep --emit-library` is the serve-layer twin of `--emit-dispatch`:

- **discover** — the top constraint re-encoded across N projects (deterministic,
  no model), with one evidence rule-line gathered per project.
- **author** — Opus writes a full `library.Entry`: the recipe artifact plus the
  groupchat-style triggering metadata (`match`/`avoid` exact-token triggers,
  `deploy_when`/`too_much_if`/`mechanism`/`key`).
- **gate** (`library.GateEntry`, deterministic, no serve) — the load-bearing
  step. The model-authored part that the gate checks is the *triggering*: the
  entry must SERVE on its realistic positives, ABSTAIN via the too-much guard
  where it should hold back, and NEVER serve on a standing set of benign,
  unrelated contexts. The recipe *body* is the fuzzy residual — but serve-time
  abstain-over-wrong bounds its blast radius.
- **propose** — a passing entry is written to `.crystal-proposals/`, never
  auto-added to the live library.

The gate earned its keep on the first live run: it rejected three *distinct* real
authoring defects before passing — (a) positives written as descriptions instead
of literal contexts the keyword matcher can fire on; (b) discovery evidence-lines
folded in as positives, which embed the remediation language an Avoid token
guards on (self-contradiction); (c) `avoid` tokens chosen as generic scope words
(`all`/`everything`) that appear in legitimate serve contexts and suppress the
entry's own positives. Each fix was a generalizable prompt constraint, not a
one-off. The honest caveat: coverage is gated against model-authored positives
(no non-authored realistic serve corpus exists yet), so the gate proves the
trigger is self-consistent and not benign-broad — it does not yet prove the
recipe body is correct (that's the g<1 fuzzy residual, by design).
