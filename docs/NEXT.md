# Working priorities (rolling)

Status: working note. Reranked 2026-06-30. Ordered by leverage against the
README vision, not by ease. Items move; this file is meant to be rewritten.

## The reranked list

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
   it's the substrate that lacks a discriminating chore. Next: author a
   middle-band chore (a small synthetic enum command) or run the executor at a tier
   between "floors" and "trivial".
4. **Make the local-open rung real (fill the deferred transfer cell).** The
   openness axis is asserted end-to-end but only measured in pieces (A5:
   qwen3.6:35b ties Haiku on categorize). Run a recipe rung through the GPU-box
   35B and report transfer fraction. Without this, "cheaper/opener/localler" has
   a hole exactly where it is most contestable.
5. **Wire `library` serve into a live hook.** Serve is demoed over a canned
   stream; make it fire on real PreToolUse events like `guard`/`dispatch` do.
6. **defn `test <name>` dispatch** — keep deferred (sibling deadline); the
   `--verifier test` raw-`go test` shim covers it for now.
7. **slimemold as the knowledge-work gate signal** — defer until the code-side
   verifier swap is solid; it is the hard (semantic) end and should not be
   load-bearing yet.

The reorder vs. before: (1) and (2) jumped to the top (were latent), pushing the
transfer-harness work down to #3 — close the loop before measuring more cells of
it.

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
