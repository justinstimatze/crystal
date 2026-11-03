"""Stream LCB jsonl on stdin; keep rows whose question_id is in ids.json; decode private tests safely."""
import sys, json, base64, zlib, pickle, io
class NoCode(pickle.Unpickler):
    def find_class(self, m, n): raise pickle.UnpicklingError(f"blocked {m}.{n}")
want = set(json.load(open(sys.argv[1]))); out = open(sys.argv[2], "a")
n = 0
for line in sys.stdin:
    if '"question_id"' not in line: continue
    r = json.loads(line)
    if r["question_id"] not in want: continue
    priv = r["private_test_cases"]
    try: priv = json.loads(priv)
    except Exception: priv = json.loads(NoCode(io.BytesIO(zlib.decompress(base64.b64decode(priv)))).load())
    r["private_test_cases"] = priv; r["public_test_cases"] = json.loads(r["public_test_cases"])
    out.write(json.dumps(r) + "\n"); n += 1
print("kept", n, file=sys.stderr)
