# team-solve runbook: operational lessons (read before running experiments)

Learned the hard way on 2026-11-03/04. Each line is something that cost time or silently corrupted results.

## Measurement
- **Grade separately from generation.** Grading inside a live run shares the CPU with everything in
  flight; a correct-but-slow program flipped from 43/43 to 3/43 under load. Run `crystal team-regrade`
  on an idle machine before reporting any number.
- **Check `cutoffs` and `finish_reason` on every run.** Caps are silent: the Hugging Face router
  capped Qwen3.8-27B at exactly 40,960 output tokens whatever `max_tokens` said; Mistral Large 4 cut
  21 of 71 calls at 64K. A capped run is not the model's score.
- **Reasoning is not on by default everywhere.** Mistral's API reasons only with `reasoning_effort`
  set (every Mistral number before 2026-11-04 was thinking-off). DeepSeek V4.1-Flash on HF needs
  `reasoning_effort` too. Qwen3.8 on nscale rejects `high` (accepts xhigh/medium/low; default xhigh).
  gpt-oss-120b on novita caps `max_tokens` at 32,768 (groq accepts 64K). Smoke-test each (model, host)
  with one small call and read the reasoning-token count before a full run.
- **A (model, host) pair is the expert, not the model.** Hosts differ in caps, accepted parameters and
  reliability; key results and calibration by both.
- **Errors count as unsolved in summaries.** Read the error column; rerun failures before comparing.

## Hosts and rate limits
- **The HF router's providers trip circuit breakers at ~12 concurrent requests** (nscale and novita
  both did: gateway time-outs, then "currently failing ... paused by the circuit breaker"). Use ≤4 per
  provider. Mistral's API took 10-14 fine.
- **The proxy drops very long requests** ("connection reset", "unexpected EOF") on 20-40 minute
  reasoning calls; retries help, and a stalled host is better swapped than hammered.

## Machine limits in cloud sessions
- **Memory: about 15 GB.** LiveCodeBench hidden tests are huge (118 problems = 3.3 GB of jsonl) and each
  `team-solve` process holds the whole file (~3-5 GB). Run at most two at once on such files.
- **Background commands are stopped at 2 hours.** Split long work into chunks under 2 hours; the disk
  cache makes a rerun of the same command resume for free.
- **Never edit a script a running launcher is executing.** Bash reads scripts as it goes; an edit
  mid-run broke a launcher. Copy to a new frozen file (`pool-pair-v3.sh`) and run the copy.
- **`pgrep -f PATTERN` matches the shell running it** when the pattern is in its own command line, so
  an `until ! pgrep -f X` waiter never ends. Use the bracket trick: `pgrep -f '[r]uns/pool-x '`.
  Better still, launch the next step yourself when a completion notice arrives instead of chaining waiters.

## Modal from a cloud session
- Install `modal[api-proxy-support]`: without `python-socks` the client cannot use the session proxy
  ("Could not connect to the Modal server").
- **Large function arguments and return values go straight to Cloudflare R2, bypassing the proxy**, and
  fail with a DNS error. Keep arguments small (build big things into the image, e.g. `run_commands("git
  clone ...")`) and return results through a Modal Volume, then `modal volume get` (which uses the proxy).
- **External code (scraped editorial solutions) is not run locally.** The permission check blocks it;
  run it on Modal with `block_network=True` (`bench/teamsolve/fresh/modal_build.py`).

## Data and secrets
- Downloaded datasets are untrusted: run readers with `python3 -I`, and decode LiveCodeBench private
  tests with an unpickler that refuses every class (`lcb_filter.py`), never `pickle.loads`.
- AtCoder submissions now need a login; editorials are public and carry official reference code.
- Temporary keys live in scratch env files only. Scan for them (`grep -rlE 'sk-ant-|mstrl_|hf_...'`)
  before persisting anything to git, Modal or Drive.

## Persistence
- The container is reclaimed: bulk artifacts (cache, runs) go to the Modal volume `crystal-teamsolve`
  under a dated folder; the Drive connector takes only small text, so Drive gets a manifest pointing
  there. Snapshot after every stage.
