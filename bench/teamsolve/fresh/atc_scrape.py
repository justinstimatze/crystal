"""Scrape AtCoder contests: English statement, samples, and editorial code blocks per task.
Writes one JSON per task into OUT. Pages are untrusted data: only parsed, never executed here."""
import sys, re, json, os, time, html, urllib.request
OUT = sys.argv[1]; contests = sys.argv[2:]
UA = {"User-Agent": "crystal-research-scraper/0.1 (polite, 1 req/s)"}
def get(u):
    time.sleep(1.0)
    req = urllib.request.Request(u, headers=UA)
    with urllib.request.urlopen(req, timeout=30) as r: return r.read().decode("utf-8", "replace")
def text(h): return html.unescape(re.sub(r"<[^>]+>", "", h)).strip()
for c in contests:
    try: tl = get(f"https://atcoder.jp/contests/{c}/tasks?lang=en")
    except Exception as e: print(c, "tasks error", e); continue
    tasks = sorted(set(re.findall(rf"/contests/{c}/tasks/({c}_[a-z0-9]+)", tl)))
    eds = {}
    try:
        el = get(f"https://atcoder.jp/contests/{c}/editorial?editorialLang=en")
        ids = sorted(set(re.findall(rf"/contests/{c}/editorial/(\d+)", el)))
    except Exception as e: ids = []; print(c, "editorial list error", e)
    for eid in ids:
        try: eh = get(f"https://atcoder.jp/contests/{c}/editorial/{eid}?lang=en")
        except Exception: continue
        m = re.search(rf"/contests/{c}/tasks/({c}_[a-z0-9]+)", eh)
        if not m: continue
        official = "Official" in eh[:20000] or "official" in eh[:20000]
        for pre in re.findall(r"<pre[^>]*>(.*?)</pre>", eh, re.S):
            code = html.unescape(re.sub(r"<[^>]+>", "", pre))
            lang = "cpp" if "#include" in code and "main" in code else ("py" if re.search(r"^(import|from|def|input\(|print\()", code, re.M) and ("input" in code or "stdin" in code) else None)
            if lang and len(code) > 80:
                eds.setdefault(m[1], []).append({"editorial": eid, "lang": lang, "official": official, "code": code})
    for t in tasks:
        try: th = get(f"https://atcoder.jp/contests/{c}/tasks/{t}?lang=en")
        except Exception as e: print(t, "task error", e); continue
        en = th.split('<span class="lang-en">', 1)
        body = en[1] if len(en) > 1 else th
        samples = []
        ins = re.findall(r'<h3>Sample Input \d+</h3>\s*<pre>(.*?)</pre>', body, re.S)
        outs = re.findall(r'<h3>Sample Output \d+</h3>\s*<pre>(.*?)</pre>', body, re.S)
        for a, b in zip(ins, outs): samples.append({"input": html.unescape(a), "output": html.unescape(b)})
        stmt_html = body.split('<div class="io-style">')[0]
        io_html = body.split('<div class="io-style">')[1] if '<div class="io-style">' in body else ""
        io_part = re.split(r'<h3>Sample Input 1</h3>', io_html)[0]
        statement = text(re.sub(r"<h3>", "\n### ", stmt_html + io_part))
        tl_m = re.search(r"Time Limit:\s*([\d.]+)\s*sec", th)
        json.dump({"id": t, "contest": c, "time_limit": float(tl_m[1]) if tl_m else 2.0, "statement": statement,
                   "samples": samples, "editorial_code": eds.get(t, [])}, open(os.path.join(OUT, t + ".json"), "w"))
        print(t, "samples", len(samples), "editorial programs", len(eds.get(t, [])), "TL", tl_m[1] if tl_m else "?")
