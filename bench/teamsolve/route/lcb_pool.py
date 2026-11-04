"""Stream LCB jsonl on stdin; keep AtCoder stdin problems of difficulty medium/hard NOT in the exclude list; decode private tests safely."""
import sys, json, base64, zlib, pickle, io
class NoCode(pickle.Unpickler):
    def find_class(self, m, n): raise pickle.UnpicklingError(f"blocked {m}.{n}")
ex = set(json.load(open(sys.argv[1]))); out = open(sys.argv[2], "a"); n = 0
for line in sys.stdin:
    r = json.loads(line)
    if r["platform"] != "atcoder" or r["difficulty"] not in ("medium", "hard") or r["question_id"] in ex: continue
    pub = json.loads(r["public_test_cases"])
    if not pub or pub[0]["testtype"] != "stdin": continue
    priv = r["private_test_cases"]
    try: priv = json.loads(priv)
    except Exception: priv = json.loads(NoCode(io.BytesIO(zlib.decompress(base64.b64decode(priv)))).load())
    r["private_test_cases"] = priv; r["public_test_cases"] = pub
    out.write(json.dumps(r) + "\n"); n += 1
print("kept", n, file=sys.stderr)
