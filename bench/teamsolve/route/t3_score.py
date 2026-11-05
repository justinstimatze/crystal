import csv, json, sys
H = json.load(open("t3/hashes.json"))
def load(fs):
    R = {}
    for f in fs:
        for r in csv.DictReader(open(f), delimiter="\t"):
            if not r["err"]: R[r["id"]] = r
    return R
E = {"deepseek": load(["runs/pool-deepseek/results-regraded.tsv", "runs/pool-deepseek-s/results-regraded.tsv"]),
     "glm": load(["runs/pool-glm/results-regraded.tsv", "runs/pool-glm-s/results-regraded.tsv"]),
     "gptoss": load(["runs/pool-gptossgroq/results-regraded.tsv"]), "opus": load(["runs/pool-opus55/results-regraded.tsv"])}
pub = lambda r: (lambda a, b: a == b and b != "0")(*r["public"].split("/"))
ids = sorted(H)
def conflict(pid, checkers):
    d = H[pid]["hashes"]["deepseek"]; frac = []
    for c in checkers:
        if c not in E or pid not in E[c] or not pub(E[c][pid]): continue  # only sample-passing checkers count
        o = H[pid]["hashes"][c]; n = sum(1 for a, b in zip(d, o) if a != b)
        frac.append(n / max(1, len(d)))
    return max(frac) if frac else 0.0, len(frac)
def auroc(s, y):
    pos = [a for a, b in zip(s, y) if b]; neg = [a for a, b in zip(s, y) if not b]
    if not pos or not neg: return float("nan")
    return sum((p > q) + .5 * (p == q) for p in pos for q in neg) / (len(pos) * len(neg))
y = [E["deepseek"][i]["solved"] != "true" for i in ids]  # label: DeepSeek false pass
print(f"DeepSeek sample-passing programs: {len(ids)}, false passes: {sum(y)}\n")
for name, ch in [("GLM only (cheap, open)", ["glm"]), ("GLM + gpt-oss (open)", ["glm", "gptoss"]), ("Opus only", ["opus"]), ("all three", ["glm", "gptoss", "opus"])]:
    sc = [conflict(i, ch)[0] for i in ids]
    flagged = [s > 0 for s in sc]
    fp_caught = sum(f and t for f, t in zip(flagged, y)); wrong = sum(f and not t for f, t in zip(flagged, y))
    print(f"  {name:24s} AUROC {auroc(sc, y):.3f}   flags {fp_caught}/{sum(y)} false passes, {wrong}/{len(ids)-sum(y)} correct programs ({100*wrong/(len(ids)-sum(y)):.0f}%)")
# cascade simulation over all 118: DeepSeek; escalate to Opus if samples fail OR conflict with checker
allids = sorted(E["deepseek"])
for name, ch, extra in [("samples only", [], []), ("samples OR conflict with GLM", ["glm"], ["glm"]), ("samples OR conflict with GLM/gpt-oss", ["glm", "gptoss"], ["glm", "gptoss"])]:
    s = c = esc = 0
    for i in allids:
        d = E["deepseek"][i]; c += float(d["cost_usd"])
        c += sum(float(E[x][i]["cost_usd"]) for x in extra if i in E[x])  # the checker's call is paid for every problem
        escalate = not pub(d) or (i in H and conflict(i, ch)[0] > 0)
        if escalate:
            esc += 1; c += float(E["opus"][i]["cost_usd"]); s += E["opus"][i]["solved"] == "true"
        else:
            s += d["solved"] == "true"
    print(f"  cascade DeepSeek > Opus, escalate on {name:36s}: {s}/118  ${c/118:.4f}/problem  escalated {esc}")
