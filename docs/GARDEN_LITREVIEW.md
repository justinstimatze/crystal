# Literature review: the "model garden" (externalized mixture of experts over heterogeneous LLMs)

2026-11-04. Condensed from a research-agent review. Tags: [V] verified against the abstract or paper
page, [S] search snippets or secondary sources, [M] from memory, unverified. Numbers are the papers'.

## The findings that change the plan

1. **Routing before generation plateaus far below the oracle.** *The Routing Plateau* (Lu et al. 2026,
   arXiv 2606.07587) [V]: 21 methods on 5 benchmarks; the top five routers within 0.22 pp of each other,
   10-30 pp below the oracle; routers mostly learn which model is best on average. 300K queries,
   bigger encoders and end-to-end fine-tuning added +1.24 pp. A tuned kNN is as good as anything
   (also Li 2025, arXiv 2505.12601 [S]). Proposed way out: use information beyond the query.
2. **Self-assessment is no better than another model's assessment; history transfers.** *Generalized
   Correctness Models* (Xiao et al. 2025, arXiv 2509.24988) [V]: a model predicting its own
   correctness is no better than an unrelated model; a correctness model trained on many models'
   histories transfers across families and datasets. Supports a separate, trained gate.
3. **Errors converge as models improve.** *Great Models Think Alike* (Goel et al. ICML 2025, arXiv
   2502.04313) [S]: mistake similarity (CAPA) rises with capability; LLM judges favor similar models.
4. **The formal planner exists, and its value is capped by the quality estimator.** *Cascade routing*
   (Dekoninck, Baader, Vechev, ICML 2025, arXiv 2410.10347, code github.com/eth-sri/cascade-routing)
   [V]: routing and cascading as budgeted optimization with a provably optimal strategy. AutoMix
   (arXiv 2310.12963) [S]: a POMDP over a small model's self-verification, >50% cost cut.
5. **Mixed-model sample allocation is directly reproducible.** *OSCA* (Zhang et al. NAACL 2025, arXiv
   2410.22480) [V]: learned allocation of samples across models/settings, beats the best single
   configuration with 128x less compute on code including LiveCodeBench. ModelSwitch (arXiv 2504.00762)
   [S]: switch model when a model's own samples disagree.
6. **Small-model ensembles can beat a frontier model, mostly through voting.** *Avengers* (Zhang et al.
   AAAI 2026, arXiv 2505.19797) [V]: ~ten 7B models, cluster routing + repeated sampling + voting, beats
   GPT-4.1 on average over 15 datasets. Avengers-Pro (arXiv 2508.12631) [S].
7. **Agent layers hurt above a strong baseline.** *Towards a Science of Scaling Agent Systems* (Kim et
   al. 2025, arXiv 2512.08296) [S]: 180 configurations; multi-agent loses 39-70% on sequential tasks,
   gains on parallelizable ones; returns turn negative once the single agent is above ~45%;
   centralized coordination contains errors best.
8. **Decomposition: only on failure.** ADaPT (Findings NAACL 2024) [S] decomposes only when the executor
   fails; Parsel (NeurIPS 2023) [S] is the strongest code evidence but predates reasoning models; MAST
   (NeurIPS 2025, arXiv 2503.13657) [S]: failures are often architectural.
9. **Hosts are experts.** Provider variance is real (gpt-oss-120b across hosts, Willison 2025-08-15;
   OpenRouter Exacto) [S]; TOPLOC (ICML 2025, arXiv 2501.16007) [S] verifies volunteer inference at low
   cost; Petals works but is slow [S].
10. **Compost works with care.** MixLLM (NAACL 2025) [S] online bandit router at ~24% of GPT-4 cost;
    Gatekeeper (ICML 2025) [S]; pitfalls: logs only show the chosen model's outcome (needs propensities),
    and a gate trained on sample-pass labels inherits the weak verifier's false passes.

Specialist LoRA pools (Arrow ICML 2024, LoraRetriever ACL 2024, MoErging survey arXiv 2408.07057) [S]:
evidence on NLU/NLG task mixtures, none on hard code at matched cost; SMoRA (2501.15103) [S] argues
LoRA routing is equivalent to one LoRA's rank partitioning. Speculative for us.

## Already built (do not reinvent)
Avengers/Avengers-Pro, cascade routing (eth-sri), OSCA, AutoMix, MixLLM, MoErging taxonomy, TOPLOC,
OpenRouter Exacto, Gas Town / Gas City. Not found anywhere: a typed calibrated gate + classical planner
over black-box experts with execution-grounded selection for code. Every component exists separately.

## Risks
1. Selection is the ceiling and is hard to raise; our pool's headroom over Opus (66 vs 64 of 73) is
   inside the noise (SE ~2.8 problems).
2. Coordination overhead eats thin margins above a strong single-call baseline.
3. Non-stationarity: providers drift, frontier releases move the oracle, calibration assumes
   exchangeability (split by date), logged feedback is one-sided.

## Experiments, ranked (falsifiers in the full review)
1. Pre-generation routers (kNN, IRT-style, Avengers-style) on the 118x7 matrix: share of the oracle gap
   closed. Expected under 30%. Cost ~$0.
2. Cross-model correctness model for after-generation selection (sample pass, model identity,
   cross-model agreement on generated inputs, code features): AUROC vs "first sample-passer".
3. Differential testing weighted by model-pair independence (CAPA), not raw votes.
4. OSCA-style allocation with k>=4 samples per model at a fixed budget, date split.
5. Cascade routing (Dekoninck) + conformal accept threshold, calibrated on earlier contests.
6. Provider-variance audit: same model, 2-3 providers, 20 problems x 3 samples, ~$5.
7. ModelSwitch for code: switch model when own samples disagree on fuzz inputs.
8. Compost gate evaluated over time: train on months 1..N, test on N+1.
