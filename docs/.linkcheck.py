"""Check internal links and #anchors in the static export (docs/out).

Usage: python3 .linkcheck.py [--external]   (run after `bun run build`)
"""

import re
import sys
import urllib.error
import urllib.request
from pathlib import Path
from urllib.parse import unquote, urljoin, urlparse

OUT = Path("out")
BASE = "/grx"
href_re = re.compile(r'<a\s[^>]*?href="([^"]+)"')
id_re = re.compile(r'\sid="([^"]+)"')
check_external = "--external" in sys.argv


def page_url(p: Path) -> str:
    rel = p.relative_to(OUT).as_posix()
    rel = rel[: -len("index.html")] if rel.endswith("index.html") else rel
    return f"{BASE}/{rel}"


def resolve(path: str) -> Path | None:
    rel = unquote(path[len(BASE):]).lstrip("/")
    for cand in (OUT / rel, OUT / rel / "index.html", OUT / f"{rel}.html"):
        if cand.is_file():
            return cand
    return None


pages = sorted(OUT.rglob("*.html"))
print(f"Pages: {len(pages)}")
ids: dict[Path, set[str]] = {}
bad: set[tuple[str, str, str]] = set()
seen_ext: set[str] = set()

for p in pages:
    src = page_url(p)
    for raw in set(href_re.findall(p.read_text(encoding="utf-8"))):
        raw = raw.replace("&amp;", "&")
        if raw.startswith(("javascript:", "mailto:", "data:")) or raw.startswith(f"{BASE}/_next/"):
            continue
        if raw.startswith(("http://", "https://")):
            u = urlparse(raw)
            if check_external and "github.com" in u.netloc and "grx-gql/grx" in u.path and raw not in seen_ext:
                seen_ext.add(raw)
                try:
                    req = urllib.request.Request(raw, method="HEAD", headers={"User-Agent": "linkcheck"})
                    urllib.request.urlopen(req, timeout=8)
                except urllib.error.HTTPError as e:
                    if e.code != 405:
                        bad.add((src, raw, f"HTTP {e.code}"))
                except Exception as e:
                    bad.add((src, raw, f"ERR {e}"))
            continue

        target = urljoin(src, raw)
        path, _, frag = target.partition("#")
        if not path.startswith(BASE):
            bad.add((src, raw, "outside base path"))
            continue
        file = resolve(path)
        if file is None:
            bad.add((src, raw, "404"))
            continue
        if frag and file.suffix == ".html":
            if file not in ids:
                ids[file] = set(id_re.findall(file.read_text(encoding="utf-8")))
            if unquote(frag) not in ids[file]:
                bad.add((src, raw, f"missing #{frag}"))

if bad:
    print(f"\nBROKEN ({len(bad)}):\n")
    for src, raw, reason in sorted(bad):
        print(f"  [{reason}] {raw}\n     on: {src}")
    sys.exit(1)
print("All checked links resolve.")
