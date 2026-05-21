#!/usr/bin/env bash
# Mirror the root CHANGELOG.md into the VitePress docs tree so the site
# always reflects the canonical changelog. The root file stays the single
# source of truth; this script adds frontmatter and strips the duplicate H1.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
SRC="$ROOT_DIR/CHANGELOG.md"
DST="$ROOT_DIR/docs/changelog.md"

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

> This page is mirrored from
> [`CHANGELOG.md`](https://github.com/patrickkabwe/grx/blob/main/CHANGELOG.md)
> at the repository root. Edit that file, not this one.

FRONTMATTER

    awk '
        BEGIN { skipped_h1 = 0 }
        !skipped_h1 && /^# / { skipped_h1 = 1; next }
        { print }
    ' "$SRC"
} > "$DST"

echo "Synced CHANGELOG.md → $DST"
