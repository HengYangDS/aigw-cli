#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
stale_snapshot_pattern='Current status|0\.1\.0-rc\.[0-9]+|codex/initial-product|GitLab SSH|GitLab API|e082b00'

reject_stale_snapshot() {
  file=$1
  if grep -n -E "$stale_snapshot_pattern" "$file"; then
    echo "release evidence contract contains a stale release snapshot" >&2
    return 1
  fi
  return 0
}

if ! sh "$root/scripts/check-release-readiness.sh" 0.1.0-rc.1 >/dev/null; then
  echo "release candidate readiness must remain packageable" >&2
  exit 1
fi

if sh "$root/scripts/check-release-readiness.sh" 0.1.0 >/dev/null 2>&1; then
  echo "unsigned GA release must fail closed" >&2
  exit 1
fi

reject_stale_snapshot "$root/docs/release-readiness.md"

fixture=$(mktemp "${TMPDIR:-/tmp}/aigw-release-readiness.XXXXXX")
trap 'rm -f "$fixture"' EXIT HUP INT TERM
cp "$root/docs/release-readiness.md" "$fixture"
printf '\nCurrent status: stale fixture\n' >> "$fixture"
if reject_stale_snapshot "$fixture" >/dev/null 2>&1; then
  echo "release readiness policy accepted a stale snapshot fixture" >&2
  exit 1
fi

echo "release readiness policy: OK"
