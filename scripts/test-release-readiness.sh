#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if ! sh "$root/scripts/check-release-readiness.sh" 0.1.0-rc.1 >/dev/null; then
  echo "release candidate readiness must remain packageable" >&2
  exit 1
fi

if sh "$root/scripts/check-release-readiness.sh" 0.1.0 >/dev/null 2>&1; then
  echo "unsigned GA release must fail closed" >&2
  exit 1
fi

if grep -n -E '当前状态（20|0\.1\.0-rc\.[0-9]+|codex/initial-product|GitLab SSH|GitLab API|e082b00' "$root/docs/release-readiness.md"; then
  echo "release evidence contract contains a stale release snapshot" >&2
  exit 1
fi

echo "release readiness policy: OK"
