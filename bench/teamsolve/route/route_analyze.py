#!/usr/bin/env python3
"""Routability analysis over per-(problem, model) team-solve results (no model calls).

    python3 -I route_analyze.py PROBLEMS.jsonl TAG=RESULTS.tsv [TAG=RESULTS.tsv ...] [--eu TAG,TAG]

For each problem we know, per model: solved (hidden tests), passed the public samples, cost, seconds.
Policies, scored by leave-one-out over problems where anything is learned:
  best-single        the single model with the most solves (chosen in-sample: an upper bound for it)
  oracle             any model in the pool solves it (perfect routing, the ceiling)
  input-router       TF-IDF of the statement + difficulty -> per-model logistic P(solve); call only argmax
  cascade            models in increasing cost order; submit the first program that passes the samples
  select-all         call every model; among sample-passers, submit the model with the best LOO solve rate
Reports solved, cost per problem and time per problem (cascade/select: sum of calls made; time for
select-all is the slowest call, as calls run in parallel)."""
import sys, csv, json, math
import numpy as np
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.linear_model import LogisticRegression

probs = {json.loads(l)["question_id"]: json.loads(l) for l in open(sys.argv[1])}
args = sys.argv[2:]; eu = set()
if "--eu" in args:
    i = args.index("--eu"); eu = set(args[i + 1].split(",")); args = args[:i] + args[i + 2:]
R = {}
for a in args:
    tag, path = a.split("=", 1)
    for r in csv.DictReader(open(path), delimiter="\t"):
        if r["err"]: continue
        a_, b_ = r["public"].split("/")
        R.setdefault(r["id"], {})[tag] = dict(solved=r["solved"] == "true", pub=(a_ == b_ and b_ != "0"),
                                             cost=float(r["cost_usd"]), sec=float(r["seconds"]))
models = sorted({m for v in R.values() for m in v})
ids = sorted(i for i in R if set(R[i]) == set(models) and i in probs)
print(f"{len(ids)} problems with all {len(models)} models: {', '.join(models)}\n")
for m in models:
    print(f"  {m:10s} solved {sum(R[i][m]['solved'] for i in ids):3d}  pass-samples {sum(R[i][m]['pub'] for i in ids):3d}"
          f"  ${np.mean([R[i][m]['cost'] for i in ids]):.4f}  median {np.median([R[i][m]['sec'] for i in ids]):.0f}s")
mean_cost = {m: np.mean([R[i][m]['cost'] for i in ids]) for m in models}

def evaluate(pool, label):
    pool = [m for m in models if m in pool]
    n = len(ids)
    best = max(pool, key=lambda m: sum(R[i][m]["solved"] for i in ids))
    out = {"best-single (" + best + ")": (sum(R[i][best]["solved"] for i in ids), mean_cost[best], np.mean([R[i][best]["sec"] for i in ids]))}
    out["oracle"] = (sum(any(R[i][m]["solved"] for m in pool) for i in ids), float("nan"), float("nan"))
    # cascade by cost
    order = sorted(pool, key=lambda m: mean_cost[m]); s = c = t = 0
    for i in ids:
        for k, m in enumerate(order):
            c += R[i][m]["cost"]; t += R[i][m]["sec"]
            if R[i][m]["pub"] or k == len(order) - 1:
                s += R[i][m]["solved"]; break
    out["cascade (cheapest first)"] = (s, c / n, t / n)
    # select-all with LOO prior; input-router with LOO logistic
    texts = [probs[i]["question_content"] + " DIFF_" + probs[i].get("difficulty", "") for i in ids]
    s_sel = s_in = c_in = t_in = 0
    for j, i in enumerate(ids):
        train = [k for k in range(n) if k != j]
        prior = {m: sum(R[ids[k]][m]["solved"] for k in train) for m in pool}
        passers = [m for m in pool if R[i][m]["pub"]]
        pick = max(passers, key=lambda m: prior[m]) if passers else max(pool, key=lambda m: prior[m])
        s_sel += R[i][pick]["solved"]
        vec = TfidfVectorizer(min_df=2, ngram_range=(1, 2), max_features=5000).fit([texts[k] for k in train])
        Xtr, Xte = vec.transform([texts[k] for k in train]), vec.transform([texts[j]])
        scores = {}
        for m in pool:
            y = [R[ids[k]][m]["solved"] for k in train]
            if len(set(y)) < 2: scores[m] = float(y[0]); continue
            scores[m] = LogisticRegression(C=1.0, max_iter=2000).fit(Xtr, y).predict_proba(Xte)[0][1]
        m = max(pool, key=lambda q: scores[q]); s_in += R[i][m]["solved"]; c_in += R[i][m]["cost"]; t_in += R[i][m]["sec"]
    out["input-router (LOO)"] = (s_in, c_in / n, t_in / n)
    out["select-all (LOO prior)"] = (s_sel, sum(R[i][m]["cost"] for i in ids for m in pool) / n,
                                     np.mean([max(R[i][m]["sec"] for m in pool) for i in ids]))
    print(f"\n{label}: {', '.join(pool)}")
    for k, (s, c, t) in out.items():
        print(f"  {k:32s} {s:3d}/{n}  " + ("" if math.isnan(c) else f"${c:.4f}/problem  mean {t:.0f}s"))

evaluate(models, "OPEN-ANY pool")
if eu: evaluate(eu, "EU-ONLY pool")
