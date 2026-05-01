#!/usr/bin/env bash
# Mirror the "Feature Parity Checklist", "Implementation Plan", and
# "Performance Requirements" sections from the root README.md into the
# Starlight content collection as the project roadmap. Also compute a
# per-section progress summary ([x] vs [ ] counts) and prepend it as a
# "Progress at a glance" table so the "what's done vs what's not"
# question is answerable without scrolling.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
SRC="$ROOT_DIR/README.md"
DST="$ROOT_DIR/docs/src/content/docs/roadmap.md"

if [[ ! -f "$SRC" ]]; then
    echo "error: $SRC not found" >&2
    exit 1
fi

mkdir -p "$(dirname "$DST")"

# 1) Slice the relevant range from the README: from "## Feature Parity
#    Checklist" through end of file (which is "## Performance Requirements"
#    today). Captured into a temp file so we can read it twice.
SLICE="$(mktemp -t grx-roadmap.XXXXXX)"
trap 'rm -f "$SLICE"' EXIT

awk '
    /^## Feature Parity Checklist/ { capture = 1 }
    capture { print }
' "$SRC" > "$SLICE"

# 2) Build the per-section progress summary by counting [x] and [ ] lines
#    under each "### " heading inside the slice.
SUMMARY_ROWS="$(awk '
    function flush(   pct) {
        if (section == "" || total == 0) return
        pct = int((done * 100) / total)
        printf "| %s | %d / %d | %d%% |\n", section, done, total, pct
    }
    /^### / {
        flush()
        section = substr($0, 5)
        sub(/[[:space:]]+$/, "", section)
        done = 0; total = 0
        next
    }
    /^## / {
        flush()
        section = ""
        next
    }
    /^- \[x\]/ { done++; total++; next }
    /^- \[ \]/ { total++; next }
    END { flush() }
' "$SLICE")"

# 3) Compute totals across all sections.
TOTAL_DONE=$(awk '/^- \[x\]/ {n++} END {print n+0}' "$SLICE")
TOTAL_ALL=$(awk '/^- \[[ x]\]/ {n++} END {print n+0}' "$SLICE")
if [[ "$TOTAL_ALL" -gt 0 ]]; then
    TOTAL_PCT=$(( TOTAL_DONE * 100 / TOTAL_ALL ))
else
    TOTAL_PCT=0
fi

# 4) Drop the "## Feature Parity Checklist" heading from the slice — the
#    Starlight page title comes from frontmatter, and we add our own
#    section headings in the rendered page.
BODY="$(awk '
    !skipped && /^## Feature Parity Checklist/ { skipped = 1; next }
    { print }
' "$SLICE")"

cat > "$DST" <<FRONTMATTER
---
title: Roadmap
description: What grx supports today, what is planned, and how the work is sequenced.
sidebar:
  order: 6
editUrl: https://github.com/patrickkabwe/grx/edit/main/README.md
tableOfContents:
  minHeadingLevel: 2
  maxHeadingLevel: 3
---

> This page is mirrored from the **Feature Parity Checklist** in
> [\`README.md\`](https://github.com/patrickkabwe/grx/blob/main/README.md)
> at the repository root. Edit that file, not this one.

## Overall progress

**${TOTAL_DONE} / ${TOTAL_ALL} items complete (${TOTAL_PCT}%)** across the
sections below.

## Progress at a glance

| Area | Done / Total | % |
| --- | --- | --- |
${SUMMARY_ROWS}

## Detailed checklist

${BODY}
FRONTMATTER

echo "Synced roadmap → $DST (${TOTAL_DONE}/${TOTAL_ALL} items, ${TOTAL_PCT}%)"
