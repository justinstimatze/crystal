#!/usr/bin/env python3
"""Round analysis over complete per-(problem, expert) results. No model calls.

    python3 -I round_analyze.py PROBLEMS.jsonl TAG=FILE[+FILE...] [TAG=...]

Later files for a tag override earlier ones by problem id (gap-fill reruns). Rows with an error are
dropped; only problems every expert completed are scored, and the per-difficulty coverage is printed
first so a biased subset is visible.
Reports: per-expert solve rate by difficulty; perfect-routing ceiling; every escalation order over
every subset of experts (accept the first program that passes the samples), as a cost/accuracy
frontier — the offline optimum a formal planner could reach with these experts and this acceptance
rule; metacognition signals scored by AUROC against hidden-test pass."""
import sys, csv, json, itertools

probs = {}
for l in open(sys.argv[1]):
    r = json.loads(l); probs[r["question_id"]] = r.get("difficulty", "?")
R = {}
for a in sys.argv[2:]:
    tag, files = a.split("=", 1)
    for f in files.split("+"):
        for r in csv.DictReader(open(f), delimiter="\t"):
            if r["err"]:
                continue
            p, t = r["public"].split("/")
            R.setdefault(tag, {})[r["id"]] = dict(solved=r["solved"] == "true", pub=(p == t and t != "0"),
                                                  cost=float(r["cost_usd"]), out=int(r["out_tok"] or 0))
experts = sorted(R)
for lvl in ("medium", "hard"):
    n = sum(1 for i in probs if probs[i] == lvl)
    print(f"coverage {lvl:6s}: " + "  ".join(f"{e} {sum(1 for i in R[e] if probs.get(i)==lvl)}/{n}" for e in experts))
ids = sorted(set.intersection(*[set(R[e]) for e in experts]) & set(probs))
print(f"\nscored problems (all experts complete): {len(ids)}  (medium {sum(probs[i]=='medium' for i in ids)}, hard {sum(probs[i]=='hard' for i in ids)})\n")
for e in experts:
    by = {l: [i for i in ids if probs[i] == l] for l in ("medium", "hard")}
    print(f"  {e:10s} solved {sum(R[e][i]['solved'] for i in ids):3d}  (medium {sum(R[e][i]['solved'] for i in by['medium'])}/{len(by['medium'])}, hard {sum(R[e][i]['solved'] for i in by['hard'])}/{len(by['hard'])})  ${sum(R[e][i]['cost'] for i in ids)/len(ids):.4f}/problem")
print(f"\n  perfect-routing ceiling: {sum(any(R[e][i]['solved'] for e in experts) for i in ids)}/{len(ids)}")

pts = []
for k in range(1, len(experts) + 1):
    for order in itertools.permutations(experts, k):
        s = c = 0
        for i in ids:
            for j, e in enumerate(order):
                c += R[e][i]["cost"]
                if R[e][i]["pub"] or j == len(order) - 1:
                    s += R[e][i]["solved"]; break
        pts.append((c / len(ids), s, " > ".join(order)))
pts.sort()
front, best = [], -1
for c, s, o in pts:
    if s > best:
        front.append((c, s, o)); best = s
print("\n  escalation frontier (accept first sample-passer; cheapest policy at each accuracy):")
for c, s, o in front:
    print(f"    {s:3d}/{len(ids)}  ${c:.4f}/problem  {o}")

def auroc(scores, labels):
    pos = [s for s, y in zip(scores, labels) if y]; neg = [s for s, y in zip(scores, labels) if not y]
    if not pos or not neg: return float("nan")
    wins = sum((p > q) + 0.5 * (p == q) for p in pos for q in neg)
    return wins / (len(pos) * len(neg))
rows = [(e, i) for e in experts for i in ids]
y = [R[e][i]["solved"] for e, i in rows]
others = lambda e, i: sum(R[o][i]["pub"] for o in experts if o != e) / max(1, len(experts) - 1)
sig = {
    "passes samples": [R[e][i]["pub"] for e, i in rows],
    "other experts pass samples (fraction)": [others(e, i) for e, i in rows],
    "shorter reply (-output tokens)": [-R[e][i]["out"] for e, i in rows],
    "labelled medium (not hard)": [probs[i] == "medium" for e, i in rows],
}
sig["samples AND others pass"] = [a + b for a, b in zip(sig["passes samples"], sig["other experts pass samples (fraction)"])]
print(f"\n  metacognition signals, AUROC for 'this program passes the hidden tests' over {len(rows)} programs ({sum(y)} pass):")
for k, v in sig.items():
    print(f"    {auroc(v, y):.3f}  {k}")
sp = [(e, i) for e, i in rows if R[e][i]["pub"]]
fp = sum(1 for e, i in sp if not R[e][i]["solved"])
print(f"\n  false passes: {fp} of {len(sp)} sample-passing programs fail the hidden tests ({100*fp/max(1,len(sp)):.0f}%)")
