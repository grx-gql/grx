#!/usr/bin/env bash
# Run tests for library packages only (no ./examples/...), requiring each
# package to meet a minimum statement coverage threshold.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

MIN="${COVER_MIN:-90}"

fail=0
while IFS= read -r pkg; do
	[[ -z "$pkg" ]] && continue
	out="$(go test -cover -count=1 "$pkg" 2>&1)" || true
	pct="$(echo "$out" | sed -n 's/.*coverage: \([0-9.]*\)% of statements.*/\1/p' | head -1)"
	if [[ -z "$pct" ]]; then
		echo "==> $pkg — could not parse coverage"
		echo "$out"
		fail=1
		continue
	fi
	if awk -v p="$pct" -v m="$MIN" 'BEGIN { exit !(p+0 >= m+0) }'; then
		echo "==> $pkg — ${pct}% (ok)"
	else
		echo "==> $pkg — ${pct}% FAIL (need ≥${MIN}%)"
		echo "$out"
		fail=1
	fi
done < <(go list ./... | grep -Ev '/examples/' | grep -v '^github\.com/grx-gql/grx/plugin$')

exit "$fail"
