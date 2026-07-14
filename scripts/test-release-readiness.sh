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

if ! sh "$root/scripts/check-release-readiness-doc.sh" "$root/docs/release-readiness.md" >/dev/null; then
  echo "release evidence contract must accept the current documentation" >&2
  exit 1
fi

scratch=$(mktemp -d "${TMPDIR:-/tmp}/aigw-release-readiness.XXXXXX")
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
cat > "$scratch/stale.md" <<'EOF'
# Release readiness

Current status (2026-07-14): stale snapshot.
EOF
if sh "$root/scripts/check-release-readiness-doc.sh" "$scratch/stale.md" >"$scratch/out" 2>"$scratch/err"; then
  echo "stale release snapshot must be rejected" >&2
  exit 1
fi
if ! grep -Fq "release evidence contract contains a stale release snapshot" "$scratch/err"; then
  cat "$scratch/err" >&2
  echo "stale release rejection must report its policy failure" >&2
  exit 1
fi

if sh "$root/scripts/check-release-readiness-doc.sh" "$scratch/missing.md" >"$scratch/out" 2>"$scratch/err"; then
  echo "missing release evidence document must be rejected" >&2
  exit 1
fi
if ! grep -Fq "cannot read release evidence contract" "$scratch/err"; then
  cat "$scratch/err" >&2
  echo "missing document rejection must report its read failure" >&2
  exit 1
fi

echo "release readiness policy: OK"
