#!/usr/bin/env bash
# Mirror the root CHANGELOG.md into the VitePress docs tree so the site
# always reflects the canonical changelog. The root file stays the single
# source of truth; this script adds frontmatter and strips the duplicate H1.
# Generated docs/changelog.md must match this output (`scripts/check-docs-changelog.sh`).

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
SRC="$ROOT_DIR/CHANGELOG.md"
# Optional: CHANGELOG_SYNC_DST=/path/file.md to write elsewhere (used by check script).
DST="${CHANGELOG_SYNC_DST:-$ROOT_DIR/docs/changelog.md}"

if [[ ! -f "$SRC" ]]; then
    echo "error: $SRC not found" >&2
    exit 1
fi

mkdir -p "$(dirname "$DST")"

{
    cat <<'FRONTMATTER'
---
title: Changelog
description: All notable changes to grx, in reverse chronological order.
outline: [2, 3]
---

> **`docs/changelog.md` is generated** — edit only the root **`CHANGELOG.md`**, then run
> **`make docs-changelog`** and commit both files together. CI enforces freshness via
> **`scripts/check-docs-changelog.sh`**.

FRONTMATTER

    awk '
        BEGIN { skipped_h1 = 0 }
        !skipped_h1 && /^# / { skipped_h1 = 1; next }
        { print }
    ' "$SRC"
} > "$DST"

echo "Synced CHANGELOG.md → $DST"
