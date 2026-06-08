# Glossary — say it so a new reader gets it (2026-06-08)

Crystal's vocabulary drifted into insider shorthand (`served-det`, `demote`,
`oracle`, `re-author`, `shift-left`). This is the alignment key: each insider term
→ a plain-English phrase a new reader understands, plus the **established name** for
the underlying move where one exists (so we borrow a recognized term instead of
inventing). Derived with `calque` (drift detection) + `lexicon` (established
pattern names) — see the bottom for how.

**Rule of thumb:** lead with the plain phrase in anything a new reader sees (the
dashboard, the README intro, the first use in a doc). The shorthand is fine *after*
it's been introduced, and fine internally.

## Core concepts

| insider term | say this instead | established name (lexicon) |
|---|---|---|
| shift-left | **hand each task to the cheapest handler that still gets it right** | cheapest-adequate; contradiction-resolution by decomposition (cheap where cheap works, expensive where it doesn't) |
| tier | **handler** (model, rule, or cache that does the work) | — |
| the menu (executor × placement) | **the options: *what* runs the work × *where* it runs** | isolating components for separate treatment |
| crystallize | **lock a repeating task into a fixed rule** | partial evaluation / specialization |
| oracle | **answer key** (what we treat as the correct label) | — |
| agreement oracle | **two-model agreement** (trust a label when two models independently agree) | triangulation of independent methods |
| abstain | **no agreement → skip** (don't force a guess) | — |
| defer / deferred-to-model | **escalate to the model** (only call the model when the rule can't) | lazy / pay-only-when-needed |
| demote / demote-on-drift | **fall back to the model when the rule stops working** | — |
| re-author | **rebuild the rules** | — |
| served deterministically / served-det | **handled by a rule (no model call)** | — |
| covered / coverage | **the share a rule can handle on its own** | — |
| gate / verifier | **the check** (does the cheap output match the answer key?) | producer-verifier asymmetry |
| drift | (keep — widely understood) **the pattern changed, the rule fell behind** | — |
| confirm tier / cascade | **tie-breaker** (ask a stronger model only on the unsure cases) | — |

## Canonical words for the drift `calque` found

`calque synonym-report` flagged the same idea written several ways. Pick ONE and
collapse the rest (a `calque vocab-allowlist` can then gate new drift):

| pick this | retire these (when they mean the same thing) |
|---|---|
| **author** (only for generating the rule table) | written, writes, write — use "write" for *files*, "author" for *rules* |
| **verify** (the check) | confirm, confirmed — reserve "confirm" for the tie-breaker tier only |
| **real** | actual |
| **whole** | fully (when it means the same) |

## Dashboard labels (what a new reader sees first)

The live flow dashboard is the most-seen surface, so it gets the plainest words:

| internal id | dashboard label |
|---|---|
| stream | **all commands** |
| served-det | **handled by rule** |
| deferred-model | **sent to model** |
| demote | **rule fell behind** |
| agree-labeled | **models agreed** |
| cloud-confirm | **cloud tie-break** |
| abstain | **models unsure** |
| reauthor | **rules rebuilt** |
| served-now | **now by rule** |
| still-deferred | **still to model** |

Internal edge ids stay stable (data layer / history / lucida bind to them); the
dashboard maps id → label for display only.

## How this was derived (reproducible)

- `calque vocab-report` / `synonym-report` over the repo surfaced the compound
  jargon (shift-left ×54, re-author ×53, demote-on-drift ×14) and the near-synonym
  drift clusters (author/written/write; confirm/verify; real/actual).
- `lexicon_read` on a plain description of the system named the established moves
  (triangulation, lazy-evaluation, cheapest-adequate, partial-evaluation,
  amortized-optimization) so renames borrow recognized terms, not new coinages.
- Re-run either tool after a big doc change; add settled compounds to a calque
  vocab allow-list so new drift trips the gate.
