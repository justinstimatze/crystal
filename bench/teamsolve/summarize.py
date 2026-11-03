#!/usr/bin/env python3
"""Summarize team-solve results.tsv files: solved per arm, paired counts vs single, sign test.

    python3 -I bench/teamsolve/summarize.py BASE.tsv [OVERRIDE.tsv ...]

Rows in a later file replace rows with the same (id, pass, arm) in earlier ones, which is how a
rerun (a higher reply cap, a retried 504) is folded into the base run. Rows with an error are
reported and counted unsolved; a pass is reported separately per pass index.
"""
import csv, math, sys
from collections import defaultdict

rows = {}
for path in sys.argv[1:]:
    for r in csv.DictReader(open(path), delimiter="\t"):
        rows[(r["id"], r["pass"], r["arm"])] = r

def sign_p(a, b):
    n = a + b
    if n == 0:
        return 1.0
    k = max(a, b)
    return min(1.0, 2 * sum(math.comb(n, i) for i in range(k, n + 1)) / 2 ** n)

by_pass = defaultdict(dict)
for (pid, p, arm), r in rows.items():
    by_pass[p].setdefault(arm, {})[pid] = r

for p in sorted(by_pass):
    arms = by_pass[p]
    print(f"pass {p}")
    for arm in ("single", "team", "team-guard"):
        if arm not in arms:
            continue
        rs = arms[arm].values()
        solved = sum(r["solved"] == "true" for r in rs)
        errs = sum(bool(r["err"]) for r in rs)
        cut = sum(int(r["cutoffs"]) > 0 for r in rs)
        cost = sum(float(r["cost_usd"]) for r in rs)
        print(f"  {arm:10s} {solved:3d}/{len(rs)}  errors {errs}  cut-off rows {cut}  cost ${cost:.2f}")
    if "single" in arms:
        for arm in ("team", "team-guard"):
            if arm not in arms:
                continue
            ids = sorted(set(arms[arm]) & set(arms["single"]))
            t = [arms[arm][i]["solved"] == "true" for i in ids]
            s = [arms["single"][i]["solved"] == "true" for i in ids]
            to = sum(x and not y for x, y in zip(t, s)); so = sum(y and not x for x, y in zip(t, s))
            print(f"  paired {arm} vs single over {len(ids)}: both {sum(x and y for x, y in zip(t, s))}, "
                  f"{arm} only {to}, single only {so}, two-sided sign test p={sign_p(to, so):.3f}")
