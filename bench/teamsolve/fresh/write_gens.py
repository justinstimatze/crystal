"""Ask Claude Opus 5.5 for one seeded input generator per scraped task (model calls only; nothing is executed here)."""
import sys, os, re, json, glob, concurrent.futures as cf
import anthropic
SRC, DST = sys.argv[1], sys.argv[2]; os.makedirs(DST, exist_ok=True)
client = anthropic.Anthropic()
GEN_PROMPT = open(os.path.join(os.path.dirname(__file__), "gen_prompt.txt")).read()
def one(p):
    t = json.load(open(p)); out = os.path.join(DST, t["id"] + ".py")
    if os.path.exists(out): return t["id"], "cached", 0
    if not t["samples"] or not t["editorial_code"]: return t["id"], "skip (no samples or no reference)", 0
    m = client.messages.create(model="claude-opus-5-5", max_tokens=16000, output_config={"effort": "high"},
                               messages=[{"role": "user", "content": GEN_PROMPT + t["statement"]}])
    if m.stop_reason == "refusal": return t["id"], "refusal", 0
    txt = "".join(b.text for b in m.content if b.type == "text")
    blk = re.findall(r"```(?:python)?\s*\n(.*?)```", txt, re.S)
    if not blk: return t["id"], "no code block", m.usage.output_tokens
    open(out, "w").write(blk[-1]); return t["id"], "ok", m.usage.output_tokens
with cf.ThreadPoolExecutor(6) as ex:
    tot = 0
    for tid, why, ot in ex.map(one, sorted(glob.glob(os.path.join(SRC, "*.json")))):
        tot += ot; print(tid, why, ot, flush=True)
    print("output tokens", tot, "≈ $%.2f" % (tot * 20 / 1e6))
