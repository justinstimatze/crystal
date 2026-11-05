"""T3: run every expert's program on generated inputs inside an isolated Modal container (no network);
return output hashes per (expert, input) so conflict between experts can be scored locally."""
import json, os, glob, modal
app = modal.App("crystal-teamsolve-t3")
image = modal.Image.debian_slim(python_version="3.12")

@app.function(image=image, cpu=2, memory=4096, timeout=1800, block_network=True)
def run_one(pid: str, gen_code: str, programs: dict) -> tuple:
    import subprocess, tempfile, hashlib
    d = tempfile.mkdtemp()
    def run(cmd, inp, t):
        try:
            r = subprocess.run(cmd, input=inp, capture_output=True, text=True, timeout=t)
            return (r.returncode == 0), r.stdout
        except subprocess.TimeoutExpired:
            return False, ""
    norm = lambda s: "\n".join(" ".join(l.split()) for l in s.strip().splitlines())
    gen = os.path.join(d, "gen.py"); open(gen, "w").write(gen_code)
    paths = {}
    for e, code in programs.items():
        p = os.path.join(d, e + ".py"); open(p, "w").write(code); paths[e] = p
    out = {e: [] for e in programs}; n = 0
    for seed, mode in [(i, "small") for i in range(1, 21)] + [(i, "large") for i in range(101, 111)]:
        ok, inp = run(["python3", "-I", gen, str(seed), mode], "", 30)
        if not ok or not inp.strip():
            continue
        n += 1
        for e, p in paths.items():
            ok, o = run(["python3", "-I", p], inp, 10)
            out[e].append(hashlib.sha256(norm(o).encode()).hexdigest()[:16] if ok else "ERR")
    return pid, n, out

@app.local_entrypoint()
def main(jobs: str, out: str):
    J = json.load(open(jobs)); res = {}
    for pid, n, o in run_one.starmap([(j["id"], j["gen"], j["programs"]) for j in J]):
        res[pid] = {"n_inputs": n, "hashes": o}; print(pid, n)
    json.dump(res, open(out, "w"))
