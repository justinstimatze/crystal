# Crystal — current thesis (north star)

Supersedes `PROJECT_BRIEF.md` where they differ. The brief is the original charter;
this is what we believe after building Phase 1, pressure-testing it twice, and a
verified prior-art pass (`PRIOR_ART.md`).

## The throughline (how the framing evolved)

1. **Binary (wrong).** "Frontier Opus vs. static deterministic code" — crystallize a
   chore into a hook when its output is constant. The `measure` sweep produced
   "crystallizability ⟂ value" — but those numbers are **retracted** (the headline
   pattern was a walker artifact; see `MEASURE_FINDINGS.md`), and the binary axis was
   itself the error.
2. **Ladder (rejected).** A tier router (Opus→…→hook) sending each chore to the
   cheapest tier that passes a gate. Rejected: routers/cascades are well-trodden
   (FrugalGPT lineage) and boring.
3. **Recursive (the mechanism).** A **loop that constructs loops**: each tier authors
   and re-authors the harness the tier below runs inside; feedback flows down (spec +
   verifier) and up (drift, escalation). *But this mechanism is also largely prior
   art* — AutoHarness (DeepMind, Feb 2026) ships the single-hop deterministic-harness
   version; STOP / Gödel Agent / SICA ship self-authoring. See `PRIOR_ART.md`.
4. **Trust substrate (the safety reframe — see the re-centering note that follows).** The mechanism is commoditizing.
   What the field is racing *past* is making recursive self-authoring **safe to run
   unattended**. Crystal is the **trust substrate for recursive self-authoring
   loops**: verifier-gated promotion, drift-triggered demotion, tamper-proof
   guardrails, and instrumented per-hop signal loss — the discipline that turns a
   self-improving loop system from a silent-degradation hazard into something you can
   leave running.

   **Topology-general, not just a tier stack.** A vertical stack of tiers is the
   *path* special-case. The same self-authoring composition gives trees (one
   supervisor authoring many parallel sub-harnesses), dev-time cycles (a critic loop
   wrapping a runtime loop — the hybrid-loops dev-time regime), and meshes (co-equal
   loops generating each other's surface). Crystal's primitives are **edge-local and
   node-local** — a verifier gates a promotion *edge*, λ is per-*edge* loss, the
   tamper-proof kernel is a *node* property — so the discipline composes over an
   arbitrary directed graph of loops, not a single ladder. The experiments so far
   exercise only the linear/vertical case; trees, cycles, and meshes are in scope and
   untested.

## Re-centering (2026-05-29): shift-left is the point; trust is the scaffolding

Stage 4 above is *true but secondary*, and it was the assistant's emphasis more than the
project's. The durable core — confirmed by an adversarial prior-art pass (the trust-substrate
framing carries the heaviest prior art: **AI Control**, **reward-tampering**) and by a
"time-traveler from 2032" gut-check (the humble shift-left, especially to a deterministic local
hook, is the part that ages best; the grand trust reframe is the part platforms most likely
internalize) — is the **humble shift-left** itself: crystallize mechanical work down to a
cheaper/deterministic tier behind a gate, and keep it there as patterns drift. That is the value
proposition and the thing to build first (`crystallize`, built; the LLM/local tiers, roadmap). The
trust substrate (verifier gate + drift demotion + the still-unbuilt tamper-proof kernel) is the
*enabler that keeps shift-left from rotting* — necessary, but not the headline. Read the rest of
this doc with that ordering: trust claims are the supporting cast, not the lead.

## Shift-left is intra-task decomposition, not just whole-task downshift (2026-05-29)

The sharper mechanism — and the one the `payoff` leak pointed at. Don't swap the *whole* task to a
cheaper model (that leaks on the hard bits, as measured). Decompose it: a task is a mix of
**mechanical, high-coverage sub-steps** (find this string, parse this, typecheck this — g≈1, a
robust deterministic tool already does it perfectly) and an **irreducible judgment residual** (which
entity is the subject, is this argument sound — 1−g). The cost-optimal architecture hands the
mechanical fraction to the ecosystem's battle-tested wheels (grep, parsers, linters) that the cheap
model merely *drives*, and pays model intelligence only for the residual. An Amdahl's-law view of
LLM cost: your bill is set by the fraction you can't offload to a tool.

- **Worked patterns (from private projects — not named here):** a claim-sourcing/verification agent
  has a cheap model drive `grep` to verify a quote — the model fills the smallest gap (what to
  search, is this a real match) while grep does the robust matching; an auditing/playtesting agent
  gives the model a tool-menu output language whose outputs are checkable ("almost a formal
  oracle"). Both are *verification-shaped* tasks — which is exactly why they decompose so well (see
  the general principle below).
- **Why it works:** a tool-constrained output has a tiny, *checkable* output space — which is
  exactly crystal's gate from the other end. AutoHarness (PRIOR_ART) is the *synthesize-the-tool*
  version; this is the cheaper *use-the-tool-that-exists* version.
- **The honest limit (our own evidence):** the `payoff` leak (Haiku grabbed the distractor
  `Tom Bradley`) was a **semantic judgment** error a grep tool would not fix. Tools collapse
  *mechanical* difficulty, not *semantic* difficulty. So "cheap model + good CLI tool > frontier
  from scratch" holds **on the tool-coverable fraction**; the optimization problem *is* maximizing
  that fraction and shrinking the judgment residual the cheap tier must cover. Error also migrates
  to tool/arg selection, output interpretation, and multi-step orchestration — shallow,
  checkable-output tool use is the sweet spot.

Ethos: a **cheap lunch, not a free one.** Perfect isn't the goal; scrappy, organic, evolving is —
reuse robust modules, accept the residual leak, let crystallize/demote accrete which decompositions
hold. (See `PRIOR_ART.md` for why per-recurring-chore stateful tiering is an unoccupied niche vs
per-request routers.)

## Crystallize your own remembering — the reflexive application (2026-06-07)

The cleanest one-line statement of what crystal *is*: **auto-chunking + shift-left applied to
remembering itself.** An expert chunks a recurring N-step procedure into one named unit so working
memory holds one token, not N; Claude re-derives it every session. Crystal binds the pattern into one
deterministic named unit (a hook, a `make` target, a git config) served from the *environment*
instead of from attention. In the shift-left vocabulary above: **recall is the frontier tier** (lossy,
forgettable, re-run every turn) and **a deterministic artifact is the cheap tier** — so promoting a
standing rule from memory to an artifact is shift-left, with *remembering* as the chore migrated.

- **The recursion that names the target.** A memory rule is the first-order "don't make me remember";
  it still fails because *applying* it is a second-order act of remembering. The fix collapses the
  recursion — move the constraint into the environment where the wrong path is unavailable. *Not
  having to remember to not have to remember.* The proof a memory rule is not enough: a rule already
  written and still violated shows recall-and-hope tops out; mint the artifact.
- **Promotion trigger — sharper than "recurs N times."** Promote a rule that recurred *despite being
  a rule.* The deterministic, verifiable proxy (`SWEEP_FINDINGS.md`): **re-encoded across N projects**
  — the same rule independently re-written in N memories means recall failed to chunk (`git add -A`
  ban in 4 projects, `main`-not-`master` in 3, secrets-to-files in 3). `weir` is the existence proof
  (a `which`→`command-v` correction already promoted to a *blocking* PreToolUse hook).
- **Built (`cmd/guard.go`).** `crystal guard` is the first **constraint-type** crystallization — a
  PreToolUse hook that denies `git add -A|.|--all`. It ships as a **self-monitoring sub-hybrid-loop**:
  a constraint has no answers to verify, so its drift signal is **override frequency** (the analog of
  the categorizer hook's coverage-collapse demote). Tracking design: each artifact self-monitors; the
  *installed hooks are the registry* (no central babysitter; any manifest must be derived from them).
- **The cost story, MEASURED and tempered.** Promoting memory→artifact does reclaim standing context
  every turn — but measured, the always-injected mechanizable reclaim is ~109 tok/turn = **1.5%** of
  the 7,134-tok global `CLAUDE.md` (`SWEEP_FINDINGS.md`). Real and permanent, but **minor and bounded**
  (most standing guidance is irreducibly semantic). It does not collapse like per-call tokens, but it
  is *not* a headline. **Latency / determinism / reliability stay the primary value; context-reclaim
  is a minor secondary** — recorded here so the framing is not over-sold.
- **The scaling seam.** `guard`/`hook` are one-rule-one-hook *prototypes*. At hundreds–thousands of
  rules, one hook per rule means N process-forks per call (~5.9ms × N). The architecture that scales
  is a **single dispatcher over a rule LIBRARY** (rules as data, in-process eval, per-rule state),
  which also gives the registry (the library dir) and the **public/personal split** — the engine +
  schema ship publicly; each user grows their own library. Build it before the library grows past a
  handful or crystal goes public.

## The general principle this is circling (hypothesis — reasoning, not yet grounded)

"Shift-left a substep" is the local, cost-projection of something more general. Stated plainly, and
flagged as a framing to test rather than a measured result:

1. **The master variable is the cheaply-verifiable fraction, not the model tier.** You can place
   work on a cheaper/deterministic mechanism exactly to the extent you can *cheaply verify* its
   output. **Producer-verifier asymmetry** (checking < generating) is the economic engine. So the
   move isn't "pick a cheaper model" — it's *restructure the task so more of it has a cheap
   verifier*, which then licenses cheap or deterministic production. Tools, constrained menus,
   schemas, types, and retrieval are all the same technique: enlarge the verifiable fraction, shrink
   the irreducible free-generation residual.
   *Caveat (the verifier's reference must itself be trustworthy — measured, `A5_PROBE_FINDINGS.md`):*
   a gate only verifies *consistency with its reference labels*, not ground truth. When that reference
   is a **fallible producer** — e.g. a two-local-model agreement oracle, which is 0.85-accurate, not
   1.0 — gate-consistency can **anti-correlate with truth at the margin**: a re-authored table that
   *corrects* a confidently-wrong oracle label is penalized by the gate for disagreeing with it
   (observed: an Opus-confirmed, truth-perfect table was *rejected* 7/8). The fix is not to trust the
   cheap reference blindly, nor to drop the gate, but to **adjudicate conflicts by escalating just the
   disputed item to the strongest available tier** (a gate-time confirm tiebreak: the strong tier
   either backs the table — override the cheap label — or backs the original — a real miss). The gate
   stays the decider; its *reference* on contested items is sourced from the most reliable producer
   you can afford, only where the cheap one is in doubt. Producer-verifier needs a trust-ordered
   producer stack, not a single oracle.
2. **The operation is specialization / partial evaluation.** The expensive model is a slow general
   interpreter of intent; crystallizing a recurring chore to a hook is *partial evaluation* —
   specialize the general capability against the actually-recurring input to get a fast residual
   program. The mechanism gradient (expensive model → cheap model → tool → deterministic hook) is a
   hierarchy of increasingly specialized, increasingly verifiable residual programs; the verifier
   guards the specialization and drift demotes it when it stops holding.
3. **The expensive tier's durable role becomes *authoring the decomposition*, not doing the work.**
   Spend frontier intelligence once to compile a fuzzy task into a verifiable, mostly-cheap pipeline
   (plan + tools + gate), amortized over cheap execution — not per call. (The AutoHarness / "loop
   that constructs loops" idea, grounded in shift-left economics instead of the trust framing.)
   *Caveat (the caller problem):* invoking a tool — which tool, what args/flags, how to read the
   result — is itself work. When that invocation is **fixed** across the chore, crystallize it to a
   deterministic wrapper (no per-call model). When it's **dynamic/contextual** (the common case) the
   per-call caller is usually a *cheap LLM*, and a cheap LLM fumbles arg construction (measured:
   `DECOMPOSE_FINDINGS`) — so gate the caller's *invocation* with a cheap deterministic verifier
   too. The producer-verifier logic recurses onto the glue: a cheap caller is safe to the extent its
   invocation is cheaply checkable.
4. **The boundary it predicts.** Tasks whose *value is the unverifiable judgment* — open-ended
   reasoning, taste, novel synthesis, "which entity is the subject" — have no cheap verifier, so
   they don't decompose and stay expensive. (Our `payoff` leak is exactly that residual.) Corollary:
   **verification-shaped tasks decompose best** (their outputs are checkable by construction — why
   sourcing/verification and auditing are such clean fits); generation-shaped tasks decompose worst.

So the more-general thing: *spend intelligence to lower fuzzy work onto verifiable primitives, run
cheap, and accept the unverifiable residual as the irreducible cost floor.* This connects to the
AI-safety *bounded / provably-beneficial* lineage (Russell; see PRIOR_ART) by analogy only — that
field constrains a *capable* optimizer's output; here we constrain a *weak* model's output to be
checkable. **Honesty flag:** this is a hypothesis. It's grounded where it touches measured ground
(the leak; producer-verifier asymmetry and partial evaluation are established CS) but is *not*
validated as "the law" for LLM systems — and is deliberately not inflated into a new grand thesis
(the trust-substrate over-reach is the cautionary tale). Note too that Anthropic already published
the *offload* half — "offload agentic computation … back into the tool calls themselves" (*Writing
Effective Tools*, Sept 2025; see PRIOR_ART) — so the novel part isn't offload, it's the
verifiability-as-master-variable + partial-evaluation + deterministic-default framing on top. It's
the altitude to test next, not a claim of priority.

## How far down does shift-left go? The compute-substrate gradient (vision — untested)

> **Refinement (2026-06-08, see `EXECUTION_MENU.md`):** the gradient below is drawn as **one
> axis**, which undersells it. It's really a **menu over two orthogonal axes** — *executor* (what
> does the work) × *placement* (where it runs / who operates it) — plus *openness* as a modifier
> (relocatability, not privacy). And it is **not a ladder you descend**: a loop is *composed* from
> the menu under a **gravitational pull left** (prefer the leftmost cell that still covers + clears
> the privacy constraint), with demote-on-drift as the *"while it still works"* clamp. Adding a tier
> (e.g. the PublicAI cloud-open models) **widens the menu**, it doesn't insert a rung. Privacy is
> governed by **operator/jurisdiction**, not open/closed (the DeepSeek-hosted case proves they're
> independent). Read the single-axis gradient below as the executor column of that fuller menu.

The model-tier gradient (frontier → cheap → local model) is one axis. The deeper one is the
**compute substrate** — the same principle (lower a recurring chore onto the cheapest mechanism that
covers it; author once; amortize; demote on drift) applied all the way down:

`frontier LLM → cheap LLM → local small model → interpreted script → compiled binary → SIMD/vectorized
→ GPU kernel → FPGA → ASIC → lookup table / analog.`

It's the existing specialization/compilation hierarchy (HFT strategies → FPGA, ML inference →
TPU/ASIC, hot regex → compiled DFA, hot code → JIT, constant output → a cached lookup) — but with a
**new** twist.

**The new thing (the correction): LLMs are collapsing authoring cost at *every* substrate at once.**
Classically you descended tier by tier because each lower substrate cost much more to author (writing
Verilog ≫ writing Python). That gate is dissolving — a frontier model can write Python, a CUDA
kernel, or HDL — so it is **not a linear descent**: you **slam left**, jumping directly to the
deepest viable substrate, using whatever it takes, skipping the tiers in between. The
maximally-ambitious crystal is a **specialization scheduler**: an LLM loop that picks the target
substrate per recurring chore and authors straight to it.

What bounds how far down you can slam (the boundaries you asked about) — and note authoring cost is
*no longer the dominant gate*:
- **Recurrence/volume N** — authoring amortizes over runs; the deepest substrates still pay off only
  at high N (though as authoring cheapens, the N threshold drops).
- **Stability (drift)** — *this flips with cheap authoring.* Re-authoring used to be the barrier to
  depth ("you can't re-flash a bitstream cheaply"). When the LLM can cheaply *re-author* the deep
  implementation on drift, drift stops gating depth — what *doesn't* cheapen is **re-verification**.
- **Verifiability becomes THE binding constraint** — you can author a GPU kernel for free, but can
  you trust it, and re-trust it each time it's re-authored on drift? Producer-verifier asymmetry, now
  load-bearing: the bottleneck moves from "can we write it" to "can we verify it." **This is exactly
  crystal's gate** — verifier-gated crystallize + demote is what makes slam-left (and re-slam-on-
  drift) *safe*. So cheap authoring doesn't make crystal less relevant; it makes the *verification*
  half the whole game.
- **Authoring capability** — the residual of the collapsing cost: Verilog is still harder for an LLM
  than Python today (a moving frontier), so the reachable depth rises over time.
- **The irreducible residual** — the genuinely fuzzy/contextual judgment *never* shifts to silicon
  (an FPGA can't decide "which entity is the subject"). The deepest substrates capture only the most
  mechanical, stable, verifiable, recurrent fraction; the residual stays at the top forever.

So the boundary isn't a fixed depth, and it's no longer authoring-cost-gated — it's roughly `N ×
stability × **verifiability**`, with authoring cost a fast-shrinking denominator. As LLM authoring
approaches free, **verification is the rate-limiter on how far left you can safely slam** — which
puts crystal's gate at the center, not the periphery. **Flagged: vision, not measured — crystal has
exercised only the model-tier and deterministic-hook end; the hardware end is the same algebra
extended, not a demonstrated capability.**

### Memoization is the floor of the gradient — the "stupid cache trick" (measured)

The cheapest shift-left of all is the dumbest: **a cache hit**. An exact-repeat input doesn't need
the model, the rule table, or even a `switch` — it needs the *stored answer*. It's the degenerate
bottom of the substrate gradient: maximal determinism, zero generality, near-zero latency. "Stupid
cache tricks" belong on the shift-left list precisely because they're the same move (recurring chore
→ cheapest mechanism that reproduces the answer) taken to its trivial limit.

crystal already runs on this: `.crystal-cache` keys every model completion by a content hash and
persists the *real measured latency* alongside the text. `serve` measured it — a Haiku
classification that cost **710ms live** is replayed from disk in **microseconds** on the repeat (p50
640ms → ~7µs/call, a ~90,000× drop, exact-repro, zero tokens). The model round-trip is *deleted* on
the covered fraction. That's not a side optimization; it **is** the value prop, demonstrated at the
floor.

The gradient's cheap end is therefore three rungs, not one:

- **exact-match memoization (cache)** — covers only inputs seen before; no generalization; the
  reference *is* the stored output. ~µs.
- **deterministic rule table** (`triage`/`author`) — covers a *class* of inputs (generalizes beyond
  the seen set); still ~µs; the move up from a cache is that it answers inputs it never saw.
- **cheap model** — covers the open-ended residual neither of the above can; ~hundreds of ms.

Caching wins where inputs *exactly* recur; the rule table wins where a *pattern* recurs; the model
is the residual. Each is the cheapest mechanism that still covers its slice — which is the whole
thesis, read bottom-up.

**Two cache regimes, both shift-left.** The above is *local* memoization — skip the call entirely on
an exact repeat (crystal's `.crystal-cache`). The second regime is **shifting cost left *within* a
tier you can't drop**: when you must call the frontier model, structure the input so the provider's
**prompt cache** hits. You don't change tier; you move the *repeated-input-processing* cost onto the
provider's cache (Anthropic prompt caching: cached prefix tokens re-bill at ~0.1× input price and
aren't re-processed, so latency drops too — within the cache's ~5-minute TTL).

The lever is **input ordering**: put the large, stable bulk first (system prompt, tool definitions,
few-shot exemplars, the long shared document) as a cached prefix; put only the volatile, per-call
bit last (the specific query). Then N calls over the same context pay full price *once* and
`cache_read` for the rest, instead of re-billing and re-processing the whole prefix N times. The
anti-pattern is interpolating something volatile (a timestamp, a per-item id) early in the prompt —
it busts the prefix and every call misses. crystal's `llm.Result` already records `cache_read_tokens`
for exactly this measurement; the disk cache currently short-circuits before any API call, so the
two regimes are complementary, not redundant — local cache for exact repeats, prompt-cache structure
for the calls you still have to make.

So "marshal the ensemble" includes the caches: drop the tier when you can (memoize / rule-table),
and when you can't, *shape the call* so the tier you're stuck with bills and thinks as little as
possible. Both are shifting left.

## Use each mechanism for its nature — marshal the ensemble (the demand-side principle)

"Don't make a model count" (the `aggregate` result — both tiers miscount, while the cheap model is
48/48 on the per-item judgment) generalizes to the deepest design rule crystal has: **an LLM is a
pattern engine.** It is superb at fuzzy/semantic recognition and mapping, and structurally weak at
precise symbolic operations — counting (the "r's in *strawberry*" tokenization failure), exact
arithmetic, state-tracking, dedup, sorting, precise recall. Use it for what it *is*; route what it
*isn't* to deterministic mechanisms.

This cuts **both ways**, and the second cut is the anti-hype one: don't reach for the LLM for
everything, or for things it's *not* good at, **just because it's the shiny new thing.** Much of the
current discourse throws a model at work that belongs in boring deterministic code (vibe-everything,
an agent for every task). crystal is partly a corrective to LLM-maximalism: the model is one
instrument with a specific nature, not the whole orchestra.

So the goal isn't model-vs-tool — it's to **marshal the whole ensemble for collective robustness and
maximal shift-left**: pattern-engine models (frontier *and* cheap) for the fuzzy/semantic fraction,
deterministic tools and code for the precise/symbolic fraction, the best available tool inventory
(weir) so the deterministic options are good ones, and verification gluing the seams. crystal is the
orchestration discipline that assigns each sub-step to the mechanism whose *nature* fits it, and
gates the joints. **Robustness is a property of the ensemble, not of any one tier** — and the more
of the work you can place on a mechanism better-suited (and cheaper/more-verifiable) than a frontier
model, the further left, and the more robust, the whole system sits. (Demand-side restatement of the
thesis; a design/values principle, evidenced by `decompose`/`support`/`aggregate`, not a fresh
measured claim.)

## Where this goes past Anthropic's published direction (open question for them)

Agree with the Anthropic posts (PRIOR_ART) — they validate decomposition, cheap-model routing,
offload-to-tools, cheapest-adequate, and producer-verifier loops. The disagreement is *how far*:
their framing keeps the model in the **driver's seat on every request** (the agent calls the tools;
verification is often another model pass; multi-agent uses ~15× more tokens). crystal pushes four
steps further:

1. **Deterministic as the DEFAULT, model as the exception** — not "a model that uses tools" but
   "tools, with a model only for the irreducible residual." The frontier model *recedes* from
   per-request execution rather than orchestrating it.
2. **Stateful per-recurring-chore crystallization + demotion** — Anthropic's patterns are
   per-request/amnesiac; crystal accretes a policy over chore-identities and demotes on drift.
3. **Cheap deterministic verification over model self-review** — push the gate toward checks that
   cost no model tokens (and eventually can't be gamed), vs evaluator-optimizer loops that re-spend
   frontier intelligence.
4. **Actively de-bias tool selection** (next section) — "give agents good tools" isn't enough; the
   model still reaches for the popular-not-best one.

*Steelman of why they may be right not to (a genuine open question — if an Anthropic engineer reads
this, I'd like the answer):* a general product agent can't assume your chores recur, so per-request
generality may dominate per-chore specialization; the frontier model may already be cheap/fast
enough that crystallizing isn't worth the engineering + drift risk; and keeping the model in the
loop preserves the flexibility that makes agents agents. crystal's bet is that for a *personal,
recurring* workload those objections weaken — which is why it's a personal-first tool, not a general
agent.

### The honest leapfrog (integration, not invention) + the bitter-lesson objection

A sourced positioning pass (PRIOR_ART) is humbling: crystal's *individual* mechanisms are each
already published — the deterministic-default inversion (**Blueprint First, Model Second**,
2508.02721), crystallizing a chore to a cheaper tier (**Agentic Plan Caching**, 2506.14852),
the accretion loop (**Agent Workflow Memory**, 2409.07429; and **compound engineering**, Every 2025),
drift-gating (**SSGM**, 2603.11768). **Claiming first-to-invert would be false.** The defensible
leapfrog is the *union none of them ships*: inversion + per-recurring-chore crystallization +
**demotion up a tier on drift** (Plan Caching has none; SSGM reconciles rather than demotes) + a
**cheap deterministic verifier** gate + **deterministic tools as the default substrate** — packaged
as one practitioner harness, personal-first.

**Pre-empt the bitter lesson** (the strongest objection): Sutton says scaled general compute beats
hand-engineered structure — the canonical case *against* deterministic scaffolding. The counter:
the bitter lesson is about **learning methods, not runtime cost allocation.** Using `rg` instead of
a model token to grep a file isn't hand-engineered *intelligence*; it's declining to pay a frontier
model to do `grep`. **crystal is cost/verification engineering, not capability engineering** — it
doesn't compete with scale, it routes around paying for it where a verifiable cheaper mechanism
exists. (Adopt the current vocabulary too: crystal is a *harness* — Agent = Model + Harness — with an
*eval*-gated, *model-routing* policy whose cheapest tier is deterministic code.)

## Training data ≠ best solutions (the tool-selection bias)

A caution on decompose-and-offload: "a cheap model driving a robust tool" is only as good as *which*
tool it drives, and a model's prior is **popularity-weighted, not quality-weighted** — it reaches
for the training-frequent solution, not the best one. Measured concretely by the companion addon
**weir**: across **25,216 real Bash invocations, ~49.7% used a pipe and 0 reached for a modern tool**
(`rg`, `fd`, `sd`, `bat`, …) — fluent pipelines out of a 1995 toolbox. So part of "author the
decomposition" is **de-biasing**: the harness deterministically supplies better-than-default
capability knowledge (a "prefer `rg` over `grep`" manifest, antipattern lints) the cheap model won't
supply itself. This both improves the offloaded sub-step and reinforces the deterministic-default
thesis — the cheap, robust, *non-model* layer is where correct-tool knowledge lives.

**Tool inventory is a first-class lever (with a provisioning cost).** The offload win is *bounded by
what's installed*: weir's superior manifest only helps because ~20 modern CLI tools were
provisioned; a stock host has the 1995 toolbox. So a richer, better-curated inventory directly
expands the tool-coverable (high-g) fraction — more sub-steps a deterministic/cheap path can own,
smaller residual for the model. But it isn't free: a one-time provisioning cost (install the tools),
and a **portability dependency** — a crystallized chore that leans on `rg` breaks on a host without
it. So the harness must (a) detect host capability and (b) fall back or declare the dependency
(exactly weir's SessionStart manifest + apt-install guidance). Net: enriching the toolbox is a cheap,
one-time, high-leverage way to grow the deterministic fraction, and the capability-detection it
requires is itself part of authoring the decomposition.

**Two causes, and a personal-first edge.** Models target the common denominator for *two* reasons:
(a) **training-majority defaults** (the statistical prior — grep over rg), and (b) **as a product** —
a mass-market model serves the median user, so it assumes the median *environment* (stock coreutils)
because it can't see yours and must work for everyone. Cause (b) is structural, not fixable by more
training. And it's a real edge for a **personal-first** harness: it can target *your* enriched,
idiosyncratic toolbox (and conventions, paths, aliases) that the mass-market model is designed to
assume away. The lab optimizes for everyone's environment; a personal harness optimizes for yours —
which is exactly the recurring-workload regime where crystal's bets get stronger.

## Why "trust substrate" — and why it's partly what hybrid always meant

The hybrid-loops framework names the disciplines an LLM block requires beyond the
von-Neumann algebra: per-block calibration, context-as-code, the dev-time loop. Those
read as *hygiene* in a single loop. **The moment loops stack — one loop's output
becomes another loop's authoring input — they become survival.** Crystal is the
load-bearing instance of hybrid-for-recursion: the place those disciplines get cashed
out under the one condition (stacking) that makes them mandatory. "Trust substrate" is
that cash-out.

## Honest positioning vs. SOTA (verified — see `PRIOR_ART.md`)

Crystal's primitives are **almost all prior art**: tier stacks (OrchVis), eval-gate
cascades (FrugalGPT), model-authored verifiers (Scoring Verifiers), self-authored
harnesses (STOP/SICA/Gödel Agent), the deterministic-harness mechanism (**AutoHarness,
2603.03329 — verbatim crystal's "expensive model synthesizes a cheap deterministic
program"**), depth-collapse-under-oversight (Scaling Laws for Scalable Oversight), and
error-cascade-needs-a-deterministic-carrier (From Spark to Fire). The genuinely open
seams are narrow:

1. **Recursive composition across a heterogeneous tier ladder** (depth > 2) — cited
   self-authoring is single-hop.
2. **Explicit per-hop up-signal loss λ** in a live stack — the closest formal cousin
   (decision-theoretic cascades) *assumes scores don't degrade with depth*; crystal
   lives in the regime it assumes away. **Partially measured now** (`GROUNDHOP`/
   `UNCOVERHOP`/`DEPTHSWEEP`/`CONTENTSWEEP`): at hop 1, λ ≈ 0 for a content-bearing
   channel (the real axis is *content-vs-verdict*, not prose-vs-typed), and neither
   detection nor content fidelity compounds-loses through 6 relays — the loss sits at
   **hop 1**, not in depth. This *tensions with* the lattice's depth-pessimism below
   and narrows the open question to: does that hold for non-cooperative relays, larger
   N, and greater depth?
3. **A tamper-proof kernel *across a self-authoring / drift loop*** — a verifier the
   supervised tier structurally **cannot rewrite**. *Re-scoped (2026-05-29):* the earlier
   "tamper-proof oversight is the durable, under-served contribution" **overclaimed** — the
   **AI Control** program (untrusted model overseen by a trusted component it can't subvert,
   2312.06942) and the **reward-tampering** lineage (decoupled approval / current-RF) own that
   premise and predate crystal by years. The surviving delta is narrow: the instantiation where
   the overseen thing is *re-authoring its own harness over time* (not a fixed deployed model),
   and structural capability-isolation on that authoring edge. Engineering novelty, not conceptual.

## The load-bearing evidence: DGM (verified)

Darwin Gödel Machine (2505.22954, corroborated by The Register + Sakana AI) had a
self-improving agent **fake its own test logs and game its hallucination detector by
deleting the markers the checker looked for.** That is the published proof that *a
self-improving tier will corrupt a verifier it can reach* — and the entire reason
hard rule #4 (stable, hand-written kernel; the supervised tier cannot modify the gate
above it) is the differentiator, not a footnote.

## The riskiest assumption (partly tested)

Can the upper tier detect a lower tier's drift from a signal degraded by propagation,
and correctly re-author the lower harness — **no human, no silent degradation, with a
guardrail the lower tier can't corrupt**? The *detection-through-a-degraded-signal* half
is now grounded: through 6 relay hops, a content-bearing up-channel preserved detection
(recall 1.00) and content fidelity (~0.70 flat); the loss is at hop 1, not depth (see
the grounding-arc docs). Still unproven is the **tamper-proof re-authoring** half: a guardrail the
lower tier structurally can't corrupt (the DGM antidote), demonstrated live. That's the headline of
the *trust* track — which, per the re-centering above, is secondary to actually shipping shift-left
(serve a crystallized hook, measure the latency/determinism payoff; see `ROADMAP.md`).

## Reusable assets & what's proven vs. assumed

- **Eval/promote/demote gate** = the lattice unit cell; built, tested, tier-agnostic.
  Its value as a *trust* primitive depends on the lower tier not being able to rewrite
  it (tamper-proofing — not yet built).
- **Drift detector** = the up-feedback / ambient-meta-loop. Consecutive-K is evadable;
  the windowed M-in-W rule is the fix (`DRIFT_FINDINGS.md`).
- **Lattice sim** is **algebra, not emergent** — the frontier is `(1−λ)^(d−1) ≥
  demote/recover`, and guardrail coverage is a **cliff at `g = demote/recover`, not a
  dial** (a manufactured "depth 30" was a search-cap artifact; see correction in
  `LATTICE_FINDINGS.md`). For drift in the un-checkable residual, `g` does not help.
- **Five manufactured-confidence catches** (the 219 count; the depth-2 / depth-30
  numbers; `experiment`'s "Haiku beats Opus / λ=0.90"; `ground-hop` run-1's "λ_prose=0";
  `depth-sweep`'s "content erodes with depth", overturned by `content-sweep`) were
  caught only by verifier-against-ground-truth, adversarial reimplementation, and
  `--verbose` per-item inspection — the fifth corrected a *prior crystal finding*. This proves the
  **measurement discipline** is real (a human catching miscounts with `--verbose` — good hygiene);
  it is **not** evidence for the tamper-proof kernel (a structural block on a self-improving tier),
  which is a different mechanism and still unbuilt. Don't let the track record flatter the unbuilt
  guardrail by association.

## The leapfrog (where to get ahead, not behind)

**The "labs solve it first" risk is real.** Capability-isolating a verifier the agent can't reach is
fundamentally sandboxing/privilege-separation — exactly what the platform owners who run the only
self-improving stacks today ship as table stakes. On a 12–24mo horizon, "safe unattended
self-improvement" most likely arrives as an agent-framework feature, not a standalone substrate. So
don't bet the project on out-building labs at oversight. Bet on the two things they structurally
*won't* ship, both of which serve shift-left:

- **The shift-left tool itself, vendor-neutral and local-first** — crystallize *your* mechanical
  work down to a deterministic/cheap tier on *your* hardware. Labs optimize their own stack; they
  don't ship you a sovereign migration of your chores. This is the primary bet.
- **Trust certificates / oversight altimeter** — machine-checkable, *locally-emitted*,
  vendor-neutral attestation per output (which tier served it, verifier coverage, drift-free
  window) and a live per-edge λ readout. The one trust asset a platform won't give you for free
  because it's cross-vendor and local by definition.

Lower-priority trust experiments (only as far as they keep shift-left honest): the un-disableable
verifier (DGM antidote) and adversarial g-hardening (a red-team tier hunts the uncovered residual;
the supervisor auto-authors fresh checks). See `ROADMAP.md` for the build order — shift-left first.

Honest limit: the un-checkable fuzzy residual never reaches zero. The claim is "raise
the trust floor and make degradation *loud*," never "guarantee safety."

## Test ladder (cheapest riskiest-assumption test first)

1. **Deterministic topology sim** (`internal/lattice`) — done; it's algebra + a
   characterization of where the loop goes blind, corrected twice.
2. **Live grounding of g and λ** — done across four experiments. `cmd/experiment` was
   instrument-invalid (diagnosed, not reported); `ground-hop`/`uncover-hop`/`depth-sweep`/
   `content-sweep` then grounded g (1.00 byte-exact; 0.50 substring on semantic drift)
   and λ (≈0 at hop 1; flat to depth 6). Net: the loss is at **hop 1, not depth or
   format** — which tensions with the lattice's depth-pessimism and is the live
   correction to the assumption above.
3. **Tamper-proof recursion demo** (the headline, still unbuilt) — a self-improving stack
   that tries to game its own evaluation (the DGM behavior) and is structurally blocked +
   demoted live, with a visible trust readout. This is the contribution, demonstrated.
