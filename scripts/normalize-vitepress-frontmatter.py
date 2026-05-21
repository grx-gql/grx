#!/usr/bin/env python3
"""Strip Starlight-specific frontmatter from docs Markdown for VitePress."""

from __future__ import annotations

import re
from pathlib import Path


def split_frontmatter(text: str) -> tuple[str, str] | None:
    if not text.startswith("---"):
        return None
    m = re.match(r"^---\r?\n(.*?)\r?\n---\r?\n", text, re.DOTALL)
    if not m:
        return None
    return m.group(1), text[m.end() :]


def line_value(fm: str, key: str) -> str | None:
    m = re.search(rf"^{re.escape(key)}:\s*(.+)$", fm, re.MULTILINE)
    return m.group(1).strip() if m else None


def toc_levels(fm: str) -> tuple[int, int]:
    m1 = re.search(r"minHeadingLevel:\s*(\d+)", fm)
    m2 = re.search(r"maxHeadingLevel:\s*(\d+)", fm)
    lo = int(m1.group(1)) if m1 else 2
    hi = int(m2.group(1)) if m2 else 3
    return lo, hi


def main() -> int:
    root = Path(__file__).resolve().parents[1] / "docs"
    skip = {".vitepress", "node_modules", "dist", "src"}
    for path in sorted(root.rglob("*.md")):
        if any(p in skip for p in path.parts):
            continue
        if path.name == "index.md" and path.parent == root:
            continue
        text = path.read_text(encoding="utf-8")
        sp = split_frontmatter(text)
        if not sp:
            continue
        fm, body = sp
        title = line_value(fm, "title")
        desc = line_value(fm, "description")
        edit_url = line_value(fm, "editUrl")
        lo, hi = toc_levels(fm)

        lines = ["---"]
        if title:
            lines.append(f"title: {title}")
        if desc:
            lines.append(f"description: {desc}")
        lines.append(f"outline: [{lo}, {hi}]")
        if edit_url is not None and edit_url.lower() == "false":
            lines.append("lastUpdated: false")
        lines.append("---")
        path.write_text("\n".join(lines) + "\n" + body, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
