#!/usr/bin/env python3
"""Create one GitHub issue per ROADMAP.md checklist section with open items.

Parses the Feature Parity Checklist (from ``## Feature Parity Checklist`` through
the line before ``## Implementation Plan``). Skips the ``### Non-goals`` section.

Usage:
    python3 scripts/gh_roadmap_tracking_issues.py           # print planned actions
    python3 scripts/gh_roadmap_tracking_issues.py --apply  # run ``gh issue create``

Requires the GitHub CLI (``gh``) authenticated for this repository when using
``--apply``. Ensures the ``roadmap`` label exists before creating issues.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ROADMAP = ROOT / "ROADMAP.md"
ROADMAP_URL = "https://github.com/grx-gql/grx/blob/main/ROADMAP.md"


def parse_sections(path: Path) -> list[tuple[str, list[str]]]:
    text = path.read_text(encoding="utf-8")
    lines = text.splitlines()

    try:
        start = next(i for i, ln in enumerate(lines) if ln.startswith("## Feature Parity Checklist"))
    except StopIteration as exc:
        raise ValueError("ROADMAP.md: missing '## Feature Parity Checklist'") from exc

    try:
        end = next(
            i for i, ln in enumerate(lines) if i > start and ln.startswith("## Implementation Plan")
        )
    except StopIteration:
        end = len(lines)

    chunk = lines[start:end]

    sections: list[tuple[str, list[str]]] = []
    current: str | None = None
    items: list[str] = []
    bullet = re.compile(r"^-\s+\[\s\]\s+(.*)$")
    heading = re.compile(r"^###\s+(.+)$")

    def flush() -> None:
        nonlocal current, items
        if current and current != "Non-goals" and items:
            sections.append((current, items))
        items = []

    for ln in chunk:
        m = heading.match(ln)
        if m:
            flush()
            current = m.group(1).strip()
            continue
        if current is None or current == "Non-goals":
            continue
        mb = bullet.match(ln)
        if mb:
            items.append("- [ ] " + mb.group(1).rstrip())
            continue
        # Continuation of the previous checklist line (indented wrap).
        if items and ln.startswith((" ", "\t")) and not ln.lstrip().startswith("#"):
            items[-1] = items[-1] + "\n" + ln.rstrip()

    flush()
    return sections


def issue_exists(title: str) -> bool:
    import json

    raw = subprocess.check_output(
        ["gh", "issue", "list", "--state", "all", "--limit", "500", "--json", "title"],
        text=True,
    )
    data = json.loads(raw)
    return any(isinstance(x, dict) and x.get("title") == title for x in data)


def ensure_roadmap_label() -> None:
    subprocess.run(
        [
            "gh",
            "label",
            "create",
            "roadmap",
            "--color",
            "0052CC",
            "--description",
            "Tracks Feature Parity Checklist work from ROADMAP.md",
        ],
        check=False,
        capture_output=True,
    )


def build_body(section: str, items: list[str]) -> str:
    joined = "\n".join(items)
    return f"""## Roadmap section

**{section}**

Open checklist items from [`ROADMAP.md`]({ROADMAP_URL}) (Feature Parity Checklist).

## Open items

{joined}

## Next steps

Use the **Roadmap / parity item** issue form for follow-up work so motivation and acceptance criteria stay consistent.
"""


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--apply",
        action="store_true",
        help="Create missing GitHub issues (requires gh auth).",
    )
    args = ap.parse_args()

    if not ROADMAP.is_file():
        print(f"error: {ROADMAP} not found", file=sys.stderr)
        sys.exit(1)

    sections = parse_sections(ROADMAP)
    if not sections:
        print("No open roadmap sections found.", file=sys.stderr)
        sys.exit(1)

    if args.apply:
        try:
            subprocess.run(["gh", "auth", "status"], check=True, capture_output=True)
        except (subprocess.CalledProcessError, FileNotFoundError) as exc:
            print("error: gh is not installed or not authenticated.", file=sys.stderr)
            sys.exit(1)
        ensure_roadmap_label()

    for section, items in sections:
        title = f"[roadmap]: {section}"
        body = build_body(section, items)
        if not args.apply:
            print("----")
            print(f"Would create: {title}")
            print(f"  ({len(items)} open item(s))")
            continue
        if issue_exists(title):
            print(f"skip (already exists): {title}")
            continue
        subprocess.run(
            [
                "gh",
                "issue",
                "create",
                "--title",
                title,
                "--body",
                body,
                "--label",
                "roadmap",
                "--label",
                "enhancement",
            ],
            check=True,
        )
        print(f"created: {title}")

    if not args.apply:
        print()
        print("Dry-run complete. Re-run with --apply to create missing issues.")


if __name__ == "__main__":
    main()
