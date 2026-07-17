#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/release-source-date-epoch.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

expect_failure() {
  name=$1
  expected=$2
  shift 2
  if "$@" >"$tmp/$name.out" 2>"$tmp/$name.err"; then
    echo "expected failure: $name" >&2
    exit 1
  fi
  grep -F "$expected" "$tmp/$name.err" >/dev/null || {
    cat "$tmp/$name.err" >&2
    echo "missing failure diagnostic for $name: $expected" >&2
    exit 1
  }
}

cat > "$tmp/valid.md" <<'DOC'
# Changelog

## [Unreleased]

## [0.1.0-rc.55] - 2026-07-17

## [0.1.0-rc.54] - 2026-07-16
DOC

actual=$("$script" 0.1.0-rc.55 "$tmp/valid.md")
[ "$actual" = 1784246400 ] || {
  echo "unexpected epoch: $actual" >&2
  exit 1
}

expect_failure missing 'release heading not found: 0.1.0-rc.56' \
  "$script" 0.1.0-rc.56 "$tmp/valid.md"

cat > "$tmp/duplicate.md" <<'DOC'
## [0.1.0-rc.55] - 2026-07-17
## [0.1.0-rc.55] - 2026-07-17
DOC
expect_failure duplicate 'release heading must occur exactly once: 0.1.0-rc.55' \
  "$script" 0.1.0-rc.55 "$tmp/duplicate.md"

cat > "$tmp/invalid-date.md" <<'DOC'
## [0.1.0-rc.55] - 2026-02-30
DOC
expect_failure invalid_date 'invalid release date: 2026-02-30' \
  "$script" 0.1.0-rc.55 "$tmp/invalid-date.md"

echo "release source-date epoch contract: OK"
