#!/usr/bin/env bash
# Ensure docs/changelog.md matches sync output derived from root CHANGELOG.md
# on disk (CI + local guardrail — no need to commit before the check passes).
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -- "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
cd "$ROOT_DIR"

EXPECTED="$(mktemp "${TMPDIR:-/tmp}/grx-changelog-sync.XXXXXX")"
cleanup() {
	rm -f "$EXPECTED"
}
trap cleanup EXIT

CHANGELOG_SYNC_DST="$EXPECTED" ./scripts/sync-changelog.sh >/dev/null

if cmp -s "$EXPECTED" docs/changelog.md 2>/dev/null; then
	exit 0
fi

echo "error: docs/changelog.md is out of sync with CHANGELOG.md" >&2
echo "hint: make docs-changelog (or ./scripts/sync-changelog.sh) then commit CHANGELOG.md and docs/changelog.md together." >&2
diff -u docs/changelog.md "$EXPECTED" >&2 || true
exit 1
