import re, urllib.request, urllib.error, sys
from pathlib import Path
from urllib.parse import urljoin, urlparse

base = "http://localhost:4321"
href_re = re.compile(r'href="([^"]+)"')
id_re = re.compile(r'\sid="([^"]+)"')

def page_url(p):
    rel = p.relative_to("dist")
    s = "/" + str(rel).replace("/index.html", "/").replace("index.html", "")
    return s

page_files = sorted(Path("dist").rglob("*.html"))
print(f"Pages: {len(page_files)}")

ids_by_page = {}
for p in page_files:
    text = p.read_text(encoding="utf-8")
    ids_by_page[page_url(p)] = set(id_re.findall(text))

bad = []
seen = set()

for p in page_files:
    src_url = page_url(p)
    text = p.read_text(encoding="utf-8")
    for raw in set(href_re.findall(text)):
        if raw.startswith(("javascript:", "mailto:", "data:")):
            continue
        if raw.startswith("/grx/_astro/") or raw == "/grx/sitemap-index.xml":
            continue
        if raw.startswith(("http://", "https://")):
            u = urlparse(raw)
            if "github.com" in u.netloc and "grx-gql/grx" in u.path:
                key = ("EXT", raw)
                if key in seen: continue
                seen.add(key)
                try:
                    req = urllib.request.Request(raw, method="HEAD",
                                                 headers={"User-Agent": "linkcheck"})
                    with urllib.request.urlopen(req, timeout=8) as r:
                        if r.status >= 400:
                            bad.append((src_url, raw, f"HTTP {r.status}"))
                except urllib.error.HTTPError as e:
                    if e.code != 405:
                        bad.append((src_url, raw, f"HTTP {e.code}"))
                except Exception as e:
                    bad.append((src_url, raw, f"ERR {e}"))
            continue

        if raw.startswith("/grx/") or raw == "/grx" or raw.startswith("/grx#"):
            target = raw
        elif raw.startswith("/"):
            target = raw
        else:
            target = urljoin(src_url, raw)

        if "#" in target:
            path, frag = target.split("#", 1)
        else:
            path, frag = target, None

        if not path:
            path = src_url
        if not path.endswith("/") and "." not in path.rsplit("/", 1)[-1]:
            path = path + "/"

        key = ("INT", path, frag)
        if key in seen: continue
        seen.add(key)

        try:
            with urllib.request.urlopen(base + path, timeout=5) as r:
                code = r.status
        except urllib.error.HTTPError as e:
            code = e.code
        except Exception as e:
            code = f"ERR {e}"

        if code != 200:
            bad.append((src_url, raw, f"HTTP {code}"))
            continue

        if frag:
            target_ids = ids_by_page.get(path)
            if target_ids is None:
                try:
                    with urllib.request.urlopen(base + path, timeout=5) as r:
                        body = r.read().decode("utf-8", "ignore")
                    target_ids = set(id_re.findall(body))
                    ids_by_page[path] = target_ids
                except Exception:
                    target_ids = set()
            if frag not in target_ids:
                bad.append((src_url, raw, f"missing #{frag}"))

if bad:
    print(f"\nBROKEN ({len(bad)}):\n")
    for src, raw, reason in sorted(set(bad)):
        print(f"  [{reason}]")
        print(f"     href: {raw}")
        print(f"     on:   {src}")
else:
    print("\nAll checked links resolve.")
