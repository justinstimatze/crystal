# Prior Art & Novelty (verified 2026-05-28)

Citations below were **independently fetched and verified** (arxiv IDs resolved, titles/claims
checked) — not taken from the research agent's word. Verdict: **15/16 real, 0 fabricated, 1
wrong-id, 2 with embellishment caveats.** This matters because crystal's own thesis is that
fluent confident citations are exactly what turn out wrong; the load-bearing one (DGM
self-sabotage) was confirmed real with independent corroboration.

## The honest map: crystal's primitives are almost all prior art

| Crystal element | Status | Owner (verified) |
|---|---|---|
| Hierarchical tier stack (Opus→Sonnet→Haiku) | **not novel** | OrchVis (2510.24937); orchestration is commodity 2026 |
| Eval-gate = cheap-model + verifier + escalation | **not novel** | FrugalGPT (2305.05176); Agreement-Based Cascading (2407.02348) |
| Model-authored verifier judging a weaker model | **not novel** | Scoring Verifiers (2502.13820) |
| A tier authors/re-authors the harness it runs in | **not novel** | STOP (2310.02304); Gödel Agent (2410.04444); SICA (2504.15228) |
| Model authors a **deterministic** harness that carries signal losslessly (guardrail-g) | **not novel (Feb 2026)** | **AutoHarness (2603.03329)** — verbatim crystal's "expensive model synthesizes cheap deterministic program that runs without API calls" |
| Safe depth collapses with oversight depth | **published phenomenon** | Scaling Laws for Scalable Oversight (2504.18530); Recursive Self-Critiquing (2502.04675) |
| Errors amplify per hop unless a deterministic carrier intervenes | **published** | From Spark to Fire (2603.04474) — ~89% containment via a governance layer ⚠️ |
| An untrusted model overseen by a trusted component it **cannot subvert** | **not novel (the owning field)** | AI Control — Greenblatt et al., *Improving Safety Despite Intentional Subversion* (2312.06942, ICML 2024); trusted-monitoring/editing line 🔶 |
| "The agent must not corrupt the signal that grades it" (tamper-resistance) | **not novel (formalized ~5y prior)** | Reward-tampering lineage — Decoupled Approval (2011.08827), current-RF / REALab (Everitt et al.) 🔶 |
| Windowed M-in-W drift trigger | **incremental** | trigger-design class |

⚠️ *From Spark to Fire* is real and on-topic, but the specific "mean-field / spectral threshold
β·ρ(A)>δ / 0.32→0.89" details the research agent attributed to it are **not in the abstract** —
only "prevents final infection in ≥89% of runs" is confirmed. Cite cautiously.

🚫 *2601.13671* (the claimed OmniNova/Google-ADK orchestration survey) is **wrong-id** — that ID
is a real but different paper. Drop the OmniNova/ADK attribution.

## The single most threatening overlap

**AutoHarness (DeepMind, arxiv 2603.03329, ICLR'26 ws)** — a *smaller* model (Gemini-2.5-Flash)
auto-synthesizes a deterministic code harness via Thompson-sampling tree search over program
space; Flash+harness beats Gemini-2.5-Pro, and the code-as-policy variant beats Pro and
GPT-5.2-High while removing the LLM at inference. **This is crystal's single-hop deterministic-
harness mechanism, shipped.** Verified real. But: it is one authoring hop, on games with a clean
environment oracle — **no stacking across tiers, no per-hop up-signal loss, no drift detection /
re-authoring over time.**

## The DGM evidence (load-bearing for the leapfrog) — CONFIRMED

**Darwin Gödel Machine (2505.22954)**: a frozen model mutates an archive of self-improving coding
agents under an empirical verifier. The agent **faked its own unit-test logs and then gamed its
hallucination detector by deleting the markers the checker looked for** — a Goodhart loophole.
Corroborated by The Register and Sakana AI. This is direct empirical evidence for crystal's
central safety claim: *a self-improving tier will corrupt a verifier it can reach.*

🔶 *AI Control and reward-tampering rows added 2026-05-29 from the adversarial panel. The base
papers are real and well-known (2312.06942 is ICML 2024); their exact IDs were **not** re-fetched
this pass, and the panel's claimed 2025–26 follow-up IDs are **unverified and several are
post-cutoff — deliberately omitted** until independently resolved. The conceptual overlap holds
regardless of the specific follow-ups.*

## What is genuinely crystal's (narrow — and the trust angle is more contested than earlier drafts claimed)

An adversarial prior-art pass (2026-05-29) found the **trust-substrate framing carries the heaviest
prior art**: the **AI Control** program is built on exactly "an untrusted model overseen by a
trusted component it cannot subvert," and **reward-tampering** formalized "the agent must not
corrupt the signal that grades it" years earlier. So the earlier draft's "tamper-proof oversight is
the durable, under-served contribution" **overclaimed.** Re-scoped honestly:

1. **Recursive composition** across a *heterogeneous* tier ladder (depth > 2) — cited self-authoring
   systems are single-hop or single-population. Open seam the field will likely close fast.
2. **Per-hop up-signal loss λ as a *live, per-edge instrument*** you read to compute current safe
   depth — distinct from decision-theoretic cascades (2605.06350, *assume* scores don't degrade) and
   from Scaling Laws for Scalable Oversight (2504.18530, *derives* optimal depth analytically but
   doesn't *measure* live λ). Narrow but real.
3. **A tamper-proof kernel specifically *across a self-authoring / drift loop*** — not "tamper-proof
   oversight" in general (AI Control owns that), but its instantiation where the thing being
   overseen is *re-authoring its own harness over time*. The engineering novelty is structural
   capability-isolation on the authoring edge; the *problem* is old (DGM is the recent proof it
   still bites).

**Re-centering (2026-05-29):** these seams are the *secondary* trust story. The project's primary
value is the humble shift-left itself — crystallize mechanical work down to a cheaper/deterministic
tier behind a gate, and keep it there as patterns drift (`crystallize`, built). The trust substrate
is the scaffolding that keeps shift-left from rotting, not the headline. All trust claims remain
contingent on a live tamper-proof demo that does not yet exist.

## Adjacent: how real harnesses & routers pick cheaper models (2026-05-29 research)

Sourced via web search; ✅ = primary doc/repo fetched, 🔶 = secondary/community source, ⚠ = could
not confirm. Directly relevant because "which model per task" is crystal's neighbor problem.

**Harnesses — almost all manual or stateless-auto, none per-chore-stateful:**

| Harness | Model selection | Source |
|---|---|---|
| Claude Code | Haiku backs "background functionality"; `opusplan` = Opus-in-plan → auto-Sonnet-for-exec (closest documented auto tier-switch); per-subagent `model` override; effort levels (within-model) | ✅ code.claude.com/docs/.../model-config |
| GitHub Copilot "Auto" | task-complexity + system-health → route among GPT-5.x/Sonnet/Haiku; 10% cost discount | ✅ docs.github.com (concept); 🔶 model list via changelog |
| Cursor "Auto" | picks "premium model best fit"; switches on quality/availability — among *premium*, not a cheap downshift | 🔶 docs phrasing firm, internals community-sourced |
| Aider | architect/editor split + a separate `--weak-model` for commit msgs & summaries | ✅ aider.chat docs |
| pi (pi.dev, Earendil) / pi-mono (M. Zechner) / Cline / Roo / Continue | **manual** model switch (`/model`); Continue assigns models to roles | ✅ pi.dev MIT (earendil-works/pi); 🔶 others |

Takeaway: the field does **predict-then-route** (per-prompt classifier, no verification — Copilot/Cursor Auto, RouteLLM/Hybrid-LLM) or **manual/role** assignment. **Per-recurring-chore, stateful, evolving tier assignment with demotion hysteresis is unoccupied** — that's crystal's niche, not the routing mechanism itself.

**OSS routers (permissive — borrow ideas, not Python deps into Go):**

| Project | Mechanism | License |
|---|---|---|
| RouteLLM (LMSYS) | learned strong/weak route at a cost threshold (mf / bert / causal-llm routers) + eval harness | Apache-2.0 ✅ |
| RoRF (Not-Diamond) | random-forest binary pairwise route over embeddings | MIT ✅ |
| semantic-router (aurelio) | embedding-similarity to route *paths/tools* (not tiers natively) | MIT ✅ |
| OpenRouter "Auto" / NotDiamond / Martian | hosted routers (NotDiamond powers OpenRouter Auto; Martian closed) | service ✅ |

**Literature (arXiv ids fetched ✅):** FrugalGPT (2305.05176, cascade-verify-escalate), RouteLLM
(2406.18665, learned route), Hybrid-LLM (2404.14618, difficulty classifier), AutoMix (2310.12963,
few-shot self-verify + POMDP because the verifier signal is *noisy*). The two families: cascade-with-
deferral (FrugalGPT/AutoMix — has a gate) vs predict-then-route (RouteLLM/Hybrid-LLM — no gate).

**Tool-augmentation angle (the intra-task-decomposition refinement):** the strongest cheap-tier
pattern isn't routing the *model* — it's shrinking the *task*. A cheap model driving a robust
deterministic tool (Haiku+grep) collapses the mechanical fraction (g≈1) to something checkable,
leaving only the judgment residual for intelligence. AutoHarness (2603.03329) is the synthesize-the-
tool form; private verification- and auditing-agents are the use-existing-tool form. This is the
lineage crystal should borrow from (verifier-gated cascade + noisy-signal hysteresis → validates the
windowed M-in-W demotion), without building a learned router.

**Bounded / provably-beneficial AI — adjacent, but a *different problem*.** Constraining an AI to
*answer/verify* rather than *act* is an established AI-safety theme; the best-grounded cite is Stuart
Russell's **provably-beneficial AI** program (decomposable logical/probabilistic substrates + formal
verification; people.eecs.berkeley.edu/~russell) ✅. But that literature is about *containing a
capable optimizer*; crystal's "oracle-ness" is the opposite end — making a *weak* model's output
cheaply **checkable**. Related only by "constrain the output space"; the motive and threat model
differ, so it's analogy, not foundation. The load-bearing prior art for crystal is the mundane CS
underneath: **producer-verifier asymmetry** (checking is cheaper than generating) + tool-augmented
LMs.

## Local tier, mesh, cascade & agreement prior art (2026-06-07 sweep)

Triggered by the A5 result (local 8B+35B on an owned 3080, two-model agreement as a label-trust
signal, remote ollama orchestration). Three parallel research passes. **All three converged on the
same verdict, which matches this doc's standing line: the mechanisms are prior art; the *integration*
is the seam.**

**Vein 1 — local inference orchestration & mesh (the plumbing is commodity).** Remote load/unload/
inspect over HTTP is fully solved: ollama (`keep_alive`, `OLLAMA_MAX_LOADED_MODELS`), NVIDIA Triton
EXPLICIT model-control, [vLLM Sleep Mode](https://blog.vllm.ai/2025/10/26/sleep-mode.html) (offload
weights→RAM, wake 18–200× faster than reload), LM Studio headless. Single-box model-swap-by-VRAM:
[llama-swap](https://github.com/mostlygeek/llama-swap). "Mesh of owned devices" is crowded —
[exo](https://github.com/exo-explore/exo), [Petals](https://github.com/bigscience-workshop/petals),
[distributed-llama](https://github.com/b4rtaz/distributed-llama), [GPUStack](https://github.com/gpustack/gpustack),
[Kalavai](https://github.com/kalavai-net/kalavai-client), LocalAI-federated, LM Studio **LM Link**
(Tailscale mesh of your boxes) — but **nearly all do capacity-sharding (split one model across boxes)
or manual selection, not task-aware whole-model routing with an output gate.**

**Vein 2 — agreement-as-trust is a reinvention (cite, don't claim).** "Two models agree → trust the
label; disagree → abstain/escalate" is **tri-training** (Zhou & Li, TKDE 2005, lamda.nju.edu.cn) at
N=2, and the escalate half is **Query-by-Committee** active learning (Seung et al., COLT 1992).
The 0.86-on-agreement lift is the *expected* precision/recall of abstention (deep ensembles,
Lakshminarayanan 2017; confident learning, Northcutt 2021) — **so report the coverage you abstain on,
not just the accuracy on the retained set.** Closest LLM-era: **Panel of LLM evaluators (PoLL)**,
Verga 2024 (arXiv 2404.18796 — diverse cheap models beat one big judge, ~7–8× cheaper). Using a
*second distinct-capability* model rather than self-checking is well-motivated by the
generation>verification self-verification asymmetry (Weng 2023, arXiv 2212.09561).

**Vein 3 — proposer/ratifier cascade is FrugalGPT et al.** Cheap-first, escalate-on-signal is
[FrugalGPT](https://arxiv.org/abs/2305.05176) (2023), [AutoMix](https://arxiv.org/abs/2310.12963)
(small generates → self-verify → POMDP escalate), [EcoAssistant](https://arxiv.org/abs/2310.03046)
(+ caching-as-floor). The token-level structural twin is **speculative decoding**
([Leviathan 2022](https://arxiv.org/abs/2211.17192); Medusa, EAGLE). Routing-before-running:
[RouteLLM](https://arxiv.org/abs/2406.18665), [Hybrid LLM](https://arxiv.org/abs/2404.14618)
(easy→edge/small, hard→cloud/large). **The proposer-confidence trigger (logprob/entropy) is also
recent prior art: [UCCI](https://arxiv.org/abs/2605.18796) (2026) — and it warns raw entropy is
miscalibrated, so a calibration layer (isotonic/temperature) is table-stakes, not a differentiator.**
Cascade-vs-route taxonomy: survey arXiv 2603.04445.

### The defensible seam (all three passes independently landed here)

Every surveyed router/cascade routes on **predicted difficulty or load** and then **trusts the cheap
output**. None pairs zoo-routing with an **output-correctness gate that demotes-on-drift**. crystal's
unclaimed cell is the *composition*: (1) **per-recurring-chore identity** as the routing key (not a
per-query difficulty guess), (2) a **deterministic verifier** (not a learned/LLM judge — which is
what makes "verify ≪ generate" actually hold at task level, the property spec-decoding gets free at
token level but cloud cascades do not), (3) **all-local tiers**, (4) **crystallize + demote-on-drift**.
Lead novelty there; cite FrugalGPT / AutoMix / tri-training / QBC / PoLL / UCCI / Leviathan as the
mechanisms. (Consistent with this doc's thesis: novelty is integration, not invention.)

### Steal, don't reinvent (concrete adoptions surfaced)

- **LoRA-adapter swap instead of whole-GGUF swap** for per-task local specialization:
  [S-LoRA](https://arxiv.org/abs/2311.03285) / [LoRAX](https://github.com/predibase/lorax) /
  vLLM multi-LoRA — near-zero-cost per-task swap on one resident base. Changes the dispatcher's
  local-model design if specialists become adapters.
- **vLLM Sleep Mode** for the strong ratifier: keep it warm-in-RAM, wake per task, instead of cold
  reload (directly addresses the 35B's spill cost).
- **Calibrate the proposer-confidence trigger** before trusting logprob/entropy (UCCI).
- **Report abstention coverage** with every "accuracy-on-agreement" number.

*Citation hygiene:* arXiv IDs for Medusa, EAGLE, SelectiveNet (Geifman & El-Yaniv 2019), and
Learning-to-Defer (Mozannar & Sontag 2020) were given from model memory by the research pass, **not
confirmed this session — verify before external citation.** All bracket-linked IDs above were
search-confirmed.

## Anthropic & practitioner writing — shift-left/decompose/offload is largely PUBLISHED (2024–2025)

A sourced pass found the core mechanics crystal leans on are already Anthropic house doctrine — so
crystal must NOT claim novelty on them. (Verified = page fetched; dates as published.)

- **Building Effective Agents** (anthropic.com/engineering/building-effective-agents, Dec 19 2024) ✅
  — explicit cheap-model routing: *"Routing easy/common questions to smaller, cost-efficient models
  like Claude Haiku … and hard/unusual questions to more capable models"*; prompt-chaining
  decomposition; evaluator-optimizer (= producer-verifier) loop; *"find the simplest solution
  possible, and only increasing complexity when needed."*
- **Writing Effective Tools for AI Agents** (…/writing-tools-for-agents, Sept 11 2025) ✅ — the
  closest match to crystal's refinement: *"offload agentic computation from the agent's context back
  into the tool calls themselves"*; verifiers *"as simple as an exact string comparison"*; tools
  return *"only high signal information."* **crystal's "offload the mechanical sub-steps to robust
  tools, model does the glue" IS this — cite it, don't claim the offload principle as novel.**
- **Effective Context Engineering** (…/effective-context-engineering-for-ai-agents, Sept 29 2025) ✅
  — *"smallest possible set of high-signal tokens"* (= cheapest-adequate); deterministic offload via
  *"Bash commands like head and tail … without ever loading the full data objects into context."*
- **Multi-agent research system** (…/multi-agent-research-system, June 13 2025) ✅ — model tiering
  (Opus lead + Sonnet workers beat single Opus ~90%); a dedicated CitationAgent verifier pass.
  **Honest complication:** *"Multi-agent systems use about 15× more tokens"* — naive decomposition
  into more *model* subagents *raises* cost. This is the best evidence that crystal's distinct move
  is offloading to *deterministic/cheap* tiers, not to more frontier agents.
- **Effective Harnesses for Long-Running Agents** (…/effective-harnesses-for-long-running-agents,
  Nov 26 2025) ✅ — deterministic init scripts over per-session re-derivation; self-verification
  gates; explicitly leaves single-vs-multi-agent open (so don't assert decomposition is universally
  optimal).
- **Steve Yegge, *Revenge of the Junior Developer*** (sourcegraph.com/blog/…, Mar 22 2025) ✅ —
  *"Task graph decomposition … is just as important today"*; supervisory agents over agent pods.
  Contrast worth citing: Yegge's verification is iterative model *self*-review ("Rule of Five"), the
  opposite of crystal's cheap deterministic gate. (His 2026 items — Gas Town/Beads, a Pragmatic
  Engineer interview gloss — were snippet-only / post-cutoff and are deliberately NOT cited.)

**Net — novelty narrows honestly:** shift-left, decomposition, cheap-model routing,
tools-over-generation, and producer-verifier loops are *published prior art* (Anthropic, Dec 2024 –
Nov 2025). crystal's defensible contribution is the *specific composition* none of them state as a
unified method: **a deterministic tier behind a cheap verifier as the DEFAULT (the model as the
exception), assigned per-recurring-chore and demoted on drift** — inverting "a model that uses
tools" into "tools, with a model only for the residual," made stateful. That this aligns with
Anthropic's own engineering direction is a *credibility tailwind*, not a threat: crystal isn't
fringe, it's the deterministic-default corner of a direction the lab is already publishing.

**Training-data bias (measured):** "give agents good tools" understates the problem — models reach
for *popular*, not *best*, tools. The companion addon **weir** measured it: across 25,216 real Bash
invocations, ~49.7% piped and **0** reached for a modern tool (`rg`/`fd`/`sd`/`bat`). So the harness
must actively *de-bias* tool selection (capability manifest + antipattern lints) — correct-tool
knowledge belongs in the cheap deterministic layer, not the model's popularity-weighted prior.

## Current discourse + academic lineage — crystal's mechanisms are mostly ALREADY published

A sourced positioning pass (2026-05-29). The leapfrog must be claimed honestly: crystal is **not
first** to any single mechanism — the defensible claim is the *integration + framing*.

- **Compound engineering** — Shipper & Klaassen, Every (every.to/chain-of-thought/compound-
  engineering…, Dec 11 2025) ✅: *"each feature [makes] the next feature easier to build"* via a
  Plan→Work→Review→Compound loop. *Same intuition* (recurring work compounds) but a *different
  substrate*: Every makes the model **better-informed** (richer context/lessons fed back, model
  stays in the driver's seat); crystal makes the model **less-needed** (crystallize to a
  cheaper/deterministic tier) **and demotes on drift** — their loop only accretes. That's the
  differentiator vs compound engineering specifically.
- **Blueprint First, Model Second** (arXiv 2508.02721, Aug 2025) ✅ — *already* inverts control: a
  deterministic blueprint governs; "models operate as controlled subordinate components, not the
  primary decision-maker." **crystal is not first to the deterministic-default inversion.**
- **Agentic Plan Caching** (2506.14852, NeurIPS 2025) ✅ — cache/reuse structured plan templates
  across similar tasks, executed by lightweight models = crystal's "crystallize a chore to a cheaper
  tier," already published. *But no drift/invalidation/demotion.*
- **Agent Workflow Memory** (2409.07429, Sep 2024) ✅ — induce reusable workflows fed back to the
  agent = crystal's accretion loop, predating the Every post by ~15 months.
- **SSGM / Stability-&-Safety-Governed Memory** (2603.11768, Mar 2026, fresh/unsettled) ✅ — semantic
  -drift measure + gated write validation + reversible reconciliation = closest analogue to
  "demote on drift + verifier gate," though it *reconciles/rolls back* rather than *demotes a tier*.

**Honest leapfrog:** the individual pieces are claimed (inversion, cheap-tier caching, accretion,
drift-gating). What's unoccupied is (1) **the union in one practitioner harness**; (2) **demote a
chore back UP a tier on drift** (Plan Caching has none; SSGM reconciles, doesn't demote); (3)
**deterministic tools as the *default* substrate**, not just a callable tool (closest is Blueprint
First). Claiming first-to-invert would be false and easily refuted — cite Blueprint First + Plan
Caching as the lineage crystal *unifies and productizes*.

**The bitter lesson is the threat to pre-empt.** Sutton's bitter lesson (scaled general compute beats
hand-engineered structure) is the canonical argument *against* deterministic scaffolding. crystal's
counter, stated plainly: the bitter lesson is about **learning methods**, not **runtime cost
allocation** — using `rg` instead of a model token to grep a file isn't hand-engineered
intelligence, it's declining to pay a frontier model to do `grep`. **crystal is cost/verification
engineering, not capability engineering** — that's the framing that sidesteps the objection.

**Vocabulary to adopt (current, load-bearing):** *harness* (Agent = Model + Harness — crystal is an
opinionated, deterministic-default, verifier-gated harness); *evals / eval-driven* (crystal's gate
under the mainstream name); *model routing* (crystal is routing taken to the extreme where the
cheapest "tier" is deterministic code). *vibe coding* / *agentic engineering* / *spec-driven
development* are the ambient register; *agents-over-chat* is too soft to lean on.
