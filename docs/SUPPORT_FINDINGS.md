# Support (residual) — where the tool can't win, does a CHEAP model cover it? (2026-05-29)

`decompose` (A4) showed that for a *tool-coverable* chore the deterministic tool wins and the model
is overhead. This is the complement: **semantic support** — does the source back the claim, often via
*paraphrase* — the uncovered residual a string tool can't match. The honest test of whether
decomposition/shift-left pays where it's supposed to.

## Method

13 (source, claim) pairs, hard labels: supported (2 verbatim + 4 paraphrase) vs not (3 contradicted,
4 unsupported). Four conditions: **opus-whole** (frontier baseline), **haiku-whole** (cheap),
**det-tool** (`rg -F -i` of the claim — verbatim only), **haiku+rtv** (cheap-LLM keyword → `rg` pulls
matching sentences → cheap model judges from that grounded evidence). Accuracy over *parsed* verdicts
only (parse-fails excluded, never defaulted — see the catch below).

## Results (verified against per-item rows; parse-fail 0 after the fix)

```
condition     accuracy    latency       paraphrase-recall (the residual, 4 items)
opus-whole    1.00 (13)   1350 ms       4/4
haiku-whole   1.00 (13)    527 ms       4/4
det-tool      0.69 (13)    ~0 ms        0/4   ← the tool cannot see paraphrase
haiku+rtv     1.00 (13)   1347 ms       4/4   (retrieved ≈ the whole short source)
```

## What it shows — completing the gradient

1. **The residual is real.** `det-tool` catches only the 2 verbatim items and **misses all 4
   paraphrase-supported** (recall 0/4; acc 0.69). A deterministic string tool genuinely cannot do
   semantic support — so "crystallize to a deterministic tier" does *not* cover this chore. This is
   the honest demonstration that the residual the model must own exists.
2. **A cheap model covers the residual as well as the frontier — here.** `haiku-whole` = `opus-whole`
   = 1.00, both 4/4 on paraphrase, and Haiku is **~2.6× faster** (527 vs 1350 ms). So where the tool
   fails, the answer is *not* "escalate to the frontier" — it's "a cheap model suffices." Shift-left
   extends into the semantic residual.
3. **Retrieval added nothing on short sources.** `haiku+rtv` matched accuracy but was the slowest
   (1347 ms: keyword-pick + judge), and its grounded evidence was ≈ the whole source every time —
   because the sources are 1–2 sentences. Retrieval's value (needle-in-a-haystack) needs *long* docs;
   untested here. On short context it's pure overhead.

**Together with `decompose`, the tier-selection rule is now demonstrated on both ends:** pick the
cheapest mechanism that *covers* the chore — deterministic tool where it can (verbatim:
`decompose`), cheap model where the tool can't (semantic: this), frontier only where the cheap model
can't (untested — see caveat).

## The seventh catch (manufactured accuracy via parse-fail default)

The first run reported `opus-whole 1.00` — but the `--verbose` `??` column showed Opus returned an
**unparseable** verdict on all 7 not-supported items: with an 8-token cap it wrote prose ("the source
does not support…") truncated before the keyword, which the confusion counter **defaulted to
"false"** = not-supported, *happening* to match the negative labels. So 7/13 of Opus's "correct"
answers were unparsed defaults masquerading as measurement. Fixed: raise the budget to 24 tokens
(Opus now emits the one-word verdict, parse-fail 0) **and** exclude parse-fails from accuracy
entirely (never default a class). Caught by the verbose dump + reasoning about the scoring logic —
the same discipline as the prior six.

## Caveats (bound the claim)

- **Easy/clear-cut items.** Haiku = Opus = 1.00 means the task did *not separate* the cheap and
  frontier tiers. Harder semantic judgments — subtle/partial support, multi-hop reasoning, long
  documents, adversarial near-misses — would likely separate them, and are where the frontier (or
  retrieval) should earn its keep. The honest claim is "a cheap model covers *easy* semantic support;
  hard semantic support is untested."
- N=13, short synthetic sources, binary supported/not, single-sample latency. `det-tool` matched the
  whole claim verbatim (a smarter deterministic check — key-phrase, fuzzy — would do better, but
  can't close the paraphrase gap in principle).
- `haiku+rtv` retrieval was a no-op on short sources; its real test (long-doc needle-finding) is the
  follow-up.

## Bottom line (easy set)

The deterministic tool can't cover semantic support (residual confirmed real), but a **cheap model
covers easy semantic support as well as the frontier at ~2.6× lower latency** — so shift-left reaches
into the residual, not just the mechanical fraction. The open question the easy set couldn't answer:
*where does the cheap model stop sufficing and the frontier (or retrieval) start earning its keep?*
That needs harder, longer, subtler items — the next experiment.

---

## Hard set (`support --hard`, 2026-05-29) — tried to separate cheap from frontier; couldn't

Long buried-needle documents (6–9 sentences) + subtle reasoning designed to break the cheap model:
quantitative traps (8% is not ">10%"; 41% is not "a majority"), scope/negation ("prioritize
automation *over* headcount growth" → not growing the workforce), partial-conjunction ("No dividend
was declared" → "declared a dividend and approved a buyback" is false), multi-hop, temporal
(former-vs-current role), overclaim ("cured"). N=13, 6 supported.

```
condition     accuracy    latency    recall on supported (6)
opus-whole    1.00 (13)   1231 ms    6/6
haiku-whole   1.00 (13)    770 ms    6/6
det-tool      0.54 (13)    ~0 ms     0/6
haiku+rtv     0.85 (13)   1470 ms    4/6   ← retrieval HURT
```

**Two honest findings, neither the one I went looking for:**

1. **The hard set did NOT separate cheap from frontier.** `haiku-whole` = `opus-whole` = 1.00 again —
   Haiku correctly handled every trap (the arithmetic comparisons, the negation/scope, the
   false conjunct, multi-hop, temporal, overclaim). So the **frontier-necessary boundary remains
   unfound**: the cheap model's reach into subtle semantic support is larger than expected. I
   designed this set to break Haiku and it didn't — either it still isn't hard enough, or this whole
   class of semantic judgment is within a cheap model's competence. Honest null result; do not claim
   "Haiku ≈ Opus everywhere" — claim "across verbatim + easy + these subtle classes, the cheap model
   matched the frontier, and I have not yet found where it doesn't."
2. **Retrieval *hurt* (0.85 < 1.00).** `haiku+rtv` missed the two buried-paraphrase items because
   lexical keyword retrieval from the *paraphrased claim* can't match the *paraphrased source*
   ("facilities" ≠ "distribution centers"; "competitor" ≠ "rival carrier"). It fed the cheap model
   *incomplete* evidence and converted right answers to wrong. **Naive lexical RAG inherits the same
   paraphrase gap that defeats `det-tool`, so decomposition-via-retrieval can be *worse* than just
   letting the cheap model read the whole document** — same family as the A4 glue-fumble (the
   retrieval/arg step is itself error-prone). Retrieval needs *semantic* matching (embeddings) or an
   anchor-entity keyword strategy to help, and only earns its keep when the doc exceeds context.

**Caveats:** the "long" docs still fit comfortably in context, so there's no lost-in-the-middle
pressure and whole-doc reading is fine for both models — retrieval's actual needle-value needs docs
that *exceed* a comfortable context window (untested). N=13. The set produced zero Haiku errors, so
it failed as a separator; finding the frontier boundary needs items that actually induce cheap-model
errors (genuinely ambiguous support, dense distractors, or reasoning chains longer than a sentence
or two).

## Combined bottom line

Across verbatim (`decompose`), easy semantic, and these subtle-reasoning classes, **a cheap model
matched the frontier** — shift-left reaches much further into the residual than expected, and the
point where the frontier becomes *necessary* is still unfound. Meanwhile both experiments show the
**decomposition glue (fragment selection in A4, lexical retrieval here) is the fragile part** — it
can degrade a cheap model that would have been right on the whole task. So: prefer whole-task cheap
model over naive decomposition when the input fits in context; reserve tool-decomposition for the
verbatim-coverable fraction (where it's exact) and for inputs too big to read whole (where retrieval
must be *semantic*, not lexical).
