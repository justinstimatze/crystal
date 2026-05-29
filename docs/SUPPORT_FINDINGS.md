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

## Bottom line

The deterministic tool can't cover semantic support (residual confirmed real), but a **cheap model
covers easy semantic support as well as the frontier at ~2.6× lower latency** — so shift-left reaches
into the residual, not just the mechanical fraction. The open question the easy set couldn't answer:
*where does the cheap model stop sufficing and the frontier (or retrieval) start earning its keep?*
That needs harder, longer, subtler items — the next experiment.
