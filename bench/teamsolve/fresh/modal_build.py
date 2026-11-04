"""Build hidden tests on Modal, isolated: compile and run judge-trusted editorial references and model-written
input generators inside a sandboxed container with no network, never on the local machine.
Run:  modal run modal_build.py --src fresh --gens gens --acl ac-library --out freshbuild/problems.jsonl"""
import json, os, glob, modal

app = modal.App("crystal-teamsolve-testbuild")
vol = modal.Volume.from_name("crystal-teamsolve")
image = (modal.Image.debian_slim(python_version="3.12").apt_install("g++", "git")
         .run_commands("git clone --depth 1 https://github.com/atcoder/ac-library /opt/acl"))

@app.function(image=image, cpu=2, memory=4096, timeout=1800, block_network=True, volumes={"/vol": vol})
def build_one(task: dict, gen_code: str) -> tuple:
    import re, hashlib, subprocess, tempfile
    d = tempfile.mkdtemp()
    def run(cmd, inp, t):
        try:
            r = subprocess.run(cmd, input=inp, capture_output=True, text=True, timeout=t)
            return r.returncode == 0, r.stdout
        except subprocess.TimeoutExpired:
            return False, ""
    norm = lambda s: "\n".join(l.strip() for l in s.strip().splitlines())
    seen, refs, why_refs = set(), [], []
    for e in task["editorial_code"]:
        if e["code"] in seen: continue
        seen.add(e["code"]); h = hashlib.sha256(e["code"].encode()).hexdigest()[:12]
        if e["lang"] == "cpp":
            src, exe = os.path.join(d, h + ".cpp"), os.path.join(d, h); open(src, "w").write(e["code"])
            r = subprocess.run(["g++", "-O2", "-std=gnu++20", "-I", "/opt/acl", src, "-o", exe], capture_output=True, text=True, timeout=300)
            if r.returncode: why_refs.append("compile error"); continue
            cmd = [exe]
        else:
            src = os.path.join(d, h + ".py"); open(src, "w").write(e["code"]); cmd = ["python3", "-I", src]
        if all((lambda o: o[0] and norm(o[1]) == norm(s["output"]))(run(cmd, s["input"], 10)) for s in task["samples"]):
            refs.append(cmd)
        else:
            why_refs.append("fails samples")
    if not refs: return task["id"], None, f"no editorial reference passes the samples ({', '.join(why_refs) or 'none'})"
    gen = os.path.join(d, "gen.py"); open(gen, "w").write(gen_code)
    tests, disagree, tried = [], 0, 0
    for seed, mode in [(i, "small") for i in range(1, 21)] + [(i, "large") for i in range(101, 111)]:
        ok, inp = run(["python3", "-I", gen, str(seed), mode], "", 30)
        if not ok or not inp.strip(): continue
        tried += 1
        outs = [run(c, inp, 10) for c in refs]
        if not all(o[0] and o[1].strip() for o in outs): continue
        if len({norm(o[1]) for o in outs}) > 1: disagree += 1; continue
        tests.append({"input": inp, "output": outs[0][1], "testtype": "stdin"})
    if tried and disagree > tried / 3: return task["id"], None, f"references disagree on {disagree}/{tried} inputs"
    if len(tests) < 10: return task["id"], None, f"only {len(tests)} usable generated tests of {tried} generated"
    row = {"question_id": task["id"], "question_content": task["statement"], "platform": "atcoder",
        "starter_code": "", "contest_id": task["contest"], "time_limit": task["time_limit"], "n_references": len(refs),
        "public_test_cases": [{"input": s["input"], "output": s["output"], "testtype": "stdin"} for s in task["samples"]],
        "private_test_cases": tests}
    os.makedirs("/vol/freshbuild", exist_ok=True)
    json.dump(row, open(f"/vol/freshbuild/{task['id']}.json", "w")); vol.commit()
    return task["id"], True, f"ok: {len(refs)} refs, {len(tests)} tests, {disagree} disagreements"

@app.local_entrypoint()
def main(src: str, gens: str, acl: str, out: str):
    jobs = []
    for p in sorted(glob.glob(os.path.join(src, "*.json"))):
        t = json.load(open(p)); g = os.path.join(gens, t["id"] + ".py")
        if os.path.exists(g): jobs.append((t, open(g).read()))
    os.makedirs(os.path.dirname(out) or ".", exist_ok=True)
    kept = 0
    with open(out + ".log", "w") as log:
        for tid, row, why in build_one.starmap(jobs, return_exceptions=False):
            print(tid, why); print(tid, why, file=log)
            if row: kept += 1
    print(f"kept {kept} of {len(jobs)}")
