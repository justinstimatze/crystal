#!/usr/bin/env python3
"""Serve an OPEN model behind an OpenAI-compatible endpoint on Modal — the
local-open rung of crystal's cost gradient, run on rented GPU instead of the
house RTX 3080 (which spills VRAM on a 35B).

Why this exists: `crystal transfer` measures the recipe ladder across executor
tiers. The local-open cell was deferred ("away from the 3080"). A Modal-hosted
vLLM OpenAI server fills it with a reliable ~32B — no VRAM-spill stall, no
manual box wake — while keeping crystal's harness authoritative (Opus authors
the recipes in Go, golden-match in Go, disk-cache in Go). Modal is only the
open-model compute behind a URL, spoken to by the same OpenAI-compatible client
that already backs the PublicAI tier.

Deploy (persistent web endpoint; container cold-starts on first request and
scales down after the idle window):

    modal deploy modal/openmodel_server.py                    # 32B default
    MODEL=Qwen/Qwen2.5-7B-Instruct GPU=L4 modal deploy \\
        modal/openmodel_server.py                             # cheap smoke test

The MODEL env var is the only knob — this endpoint is model-agnostic, so it can
push the local-open cell UP to a Sonnet-class open model (e.g. GLM-5.2, a strong
open option; note its Chinese origin is a governance/sovereignty consideration
for some users). A Sonnet-class MoE that large needs a multi-GPU deploy
(GPU="H100:8" or similar) and is not cheap to hold — a deliberate run, not a
smoke test — but the crystal-side harness needs no change: it is still just an
OpenAI-compatible URL the transfer/serve tiers speak to.

The printed URL is the OpenAI base (append /v1). Auth: Bearer of the
VLLM_API_KEY in the `crystal-vllm` Modal secret (same value crystal reads from
.env as MODAL_VLLM_KEY). Release the GPU when done:

    modal app stop crystal-openmodel
"""
import os
import subprocess

import modal

# Model + GPU are env-selectable at deploy time so the same file smoke-tests on
# a cheap 7B/L4 and then runs the real ~32B measurement on an 80GB card.
MODEL = os.environ.get("MODEL", "Qwen/Qwen2.5-32B-Instruct")
GPU = os.environ.get("GPU", "H100")
VLLM_PORT = 8000

# Pin vLLM; flashinfer speeds up the attention kernels but is optional.
vllm_image = (
    modal.Image.debian_slim(python_version="3.12")
    .pip_install("vllm==0.6.6.post1", "huggingface_hub[hf_transfer]==0.27.1")
    .env({"HF_HUB_ENABLE_HF_TRANSFER": "1", "VLLM_MODEL": MODEL})
)

# Weights are downloaded once into a persistent Volume, then reused across cold
# starts — the 65GB pull for a 32B happens a single time.
hf_cache = modal.Volume.from_name("crystal-hf-cache", create_if_missing=True)
vllm_cache = modal.Volume.from_name("crystal-vllm-cache", create_if_missing=True)

app = modal.App("crystal-openmodel")


@app.function(
    image=vllm_image,
    gpu=GPU,
    volumes={"/root/.cache/huggingface": hf_cache, "/root/.cache/vllm": vllm_cache},
    secrets=[modal.Secret.from_name("crystal-vllm")],
    timeout=30 * 60,
    scaledown_window=5 * 60,  # release the GPU 5 min after the last request
    max_containers=1,
)
@modal.web_server(port=VLLM_PORT, startup_timeout=15 * 60)
def serve():
    # Read the model from the image-BAKED env var, not the module global: this
    # function body runs in the remote container, where the deploy-time MODEL
    # shell var does not exist (the global would silently fall back to the 32B
    # default and OOM a small GPU). VLLM_MODEL is baked into the image below.
    model = os.environ["VLLM_MODEL"]
    cmd = [
        "vllm",
        "serve",
        model,
        "--host",
        "0.0.0.0",
        "--port",
        str(VLLM_PORT),
        "--api-key",
        os.environ["VLLM_API_KEY"],
        "--max-model-len",
        "8192",
        "--gpu-memory-utilization",
        "0.92",
        # vLLM 0.6.x's ZMQ IPC frontend hits ENOTSUP under Modal's runtime;
        # route around it with the in-process frontend.
        "--disable-frontend-multiprocessing",
    ]
    subprocess.Popen(" ".join(cmd), shell=True)
