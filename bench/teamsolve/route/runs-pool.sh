#!/bin/bash
# Routability data: one single call per (problem, model), reasoning on, on older LCB problems not used in any evaluation.
S=${S:?set S to a scratch dir holding hf.env, mistral.env, route-pool.jsonl and the crystal binary as ./crystal7}; cd $S
set -a; . ./hf.env; . ./mistral.env; set +a
export PUBLICAI_TIMEOUT_S=3000
PR="--price deepseek-ai/DeepSeek-V4.1-Flash:deepinfra=0.2/0.6 --price Qwen/Qwen3.8-27B:nscale=0.2/2.5 --price zai-org/GLM-5.3-Flash:novita=0.15/0.5 --price MiniMaxAI/MiniMax-M3.1-Flash:novita=0.15/0.6 --price openai/gpt-oss-120b:novita=0.05/0.25 --price mistral-small-2603=0.15/0.6 --price mistral-large-4=1.36/4.18"
run() { # tag model cap base key conc
  ./crystal7 team-solve --problems route-pool.jsonl --arms single --effort high --max-tokens $3 --base-url $4 --key-env $5 --model $2 $PR --concurrency $6 --out runs/pool-$1 --cache-dir cache > runs/pool-$1.log 2>&1 &
}
H=https://router.huggingface.co/v1; M=https://api.mistral.ai/v1
run deepseek  deepseek-ai/DeepSeek-V4.1-Flash:deepinfra 64000 $H HF_TOKEN 12
run qwen      Qwen/Qwen3.8-27B:nscale                   64000 $H HF_TOKEN 12
run glm       zai-org/GLM-5.3-Flash:novita              64000 $H HF_TOKEN 12
run minimax   MiniMaxAI/MiniMax-M3.1-Flash:novita       64000 $H HF_TOKEN 12
run gptoss    openai/gpt-oss-120b:novita                32768 $H HF_TOKEN 12
run small4    mistral-small-2603                        64000 $M MISTRAL_API_KEY 10
run large4    mistral-large-4                           128000 $M MISTRAL_API_KEY 14
wait; echo ALL DONE
