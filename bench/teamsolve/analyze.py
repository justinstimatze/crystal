#!/usr/bin/env python3
"""Post-hoc analyses over regraded team-solve results (no model calls).

    python3 -I bench/teamsolve/analyze.py cascade RESULTS.tsv [more.tsv ...]
    python3 -I bench/teamsolve/analyze.py overlap A.tsv B.tsv

cascade: run the single call first; submit it if it passes every public sample, otherwise escalate
to an arm. Reports solved and cost against always-single and always-arm, per pass. The single
call's cost is paid either way; the arm's only on escalation.
overlap: per problem, which of two runs' arms solved it (any pass); a CAPA-style view of whether
pooling two model families could raise the ceiling.
"""
import csv, sys
from collections import defaultdict

def load(paths):
    rows = {}
    for p in paths:
        for r in csv.DictReader(open(p), delimiter="\t"):
            rows[(r["id"], r["pass"], r["arm"])] = r
    return rows

def pub_all(r):
    a, b = r["public"].split("/")
    return a == b and b != "0"

def cascade(paths):
    rows = load(paths)
    passes = sorted({k[1] for k in rows})
    arms = sorted({k[2] for k in rows} - {"single"})
    for p in passes:
        ids = sorted({k[0] for k in rows if k[1] == p and (k[0], p, "single") in rows})
        for arm in arms:
            ids_a = [i for i in ids if (i, p, arm) in rows]
            if not ids_a:
                continue
            s_sol = s_cost = a_sol = a_cost = c_sol = c_cost = esc = 0
            for i in ids_a:
                s, a = rows[(i, p, "single")], rows[(i, p, arm)]
                s_ok, a_ok = s["solved"] == "true", a["solved"] == "true"
                sc, ac = float(s["cost_usd"]), float(a["cost_usd"])
                s_sol += s_ok; a_sol += a_ok; s_cost += sc; a_cost += ac
                if pub_all(s):
                    c_sol += s_ok; c_cost += sc
                else:
                    esc += 1; c_sol += a_ok; c_cost += sc + ac
            n = len(ids_a)
            print(f"pass {p} {arm:11s} n={n}: single {s_sol} (${s_cost:.2f}) | {arm} {a_sol} (${a_cost:.2f}) | "
                  f"cascade {c_sol} (${c_cost:.2f}, escalated {esc})")

def overlap(a_path, b_path):
    def solved(path):
        out = defaultdict(set)
        for r in csv.DictReader(open(path), delimiter="\t"):
            if r["solved"] == "true":
                out[r["id"]].add(r["arm"])
        return out
    a, b = solved(a_path), solved(b_path)
    A, B = set(a), set(b)
    print(f"A solved {len(A)}, B solved {len(B)}, both {len(A & B)}, only A {len(A - B)}, only B {len(B - A)}, "
          f"union {len(A | B)}")
    print("only B:", sorted(B - A))

if __name__ == "__main__":
    {"cascade": lambda: cascade(sys.argv[2:]), "overlap": lambda: overlap(*sys.argv[2:4])}[sys.argv[1]]()
