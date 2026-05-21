#!/usr/bin/env bash
# Wrapper for scripts/gh_roadmap_tracking_issues.py (see that file for details).
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
exec python3 "$SCRIPT_DIR/gh_roadmap_tracking_issues.py" "$@"
