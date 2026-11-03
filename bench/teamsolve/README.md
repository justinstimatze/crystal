# team-solve bench data

`crystal team-solve` reads a jsonl of LiveCodeBench problems. Build it from the
public dataset with the 100 ids GVS5H's harness and memdoor's bench use
(`ids.json`, copied from memdoor `examples/workflows/team-solve/bench/ids.json`,
Apache-2.0):

    for f in test5.jsonl test6.jsonl; do
      curl -sSL -H "Authorization: Bearer $HF_TOKEN" \
        https://huggingface.co/datasets/livecodebench/code_generation_lite/resolve/main/$f |
        python3 -I bench/teamsolve/lcb_filter.py bench/teamsolve/ids.json problems.jsonl
    done

About 700 MB streams through and 260 MB is kept, 100 problems. The jsonl is not
committed: its hidden tests are the benchmark. `lcb_filter.py` decodes the
private tests with an unpickler that refuses every class, so a tampered dataset
cannot run code; LiveCodeBench's own loader calls `pickle.loads`.

73 problems are AtCoder (stdin); the 27 LeetCode problems need a function-call
harness and are skipped for now.

    HF_TOKEN=… crystal team-solve --problems problems.jsonl                         # Apertus 1.5 70B, all arms
    HF_TOKEN=… crystal team-solve --problems problems.jsonl --model Qwen/Qwen3.8-27B --no-think
