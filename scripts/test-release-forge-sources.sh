#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
manifest="$root/packaging/release/forge-sources.env"

"$root/scripts/check-release-forge-sources.sh" >/dev/null
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

cp "$manifest" "$tmp/manifest"
printf 'AIGW_GITHUB_RELEASE_ORIGIN=https://duplicate.example.test\n' >> "$tmp/manifest"
if AIGW_RELEASE_FORGE_SOURCES_FILE="$tmp/manifest" "$root/scripts/check-release-forge-sources.sh" > "$tmp/duplicate.out" 2>&1; then
  echo "forge-source checker accepted a duplicate key" >&2
  exit 1
fi
grep -F 'must define AIGW_GITHUB_RELEASE_ORIGIN exactly once' "$tmp/duplicate.out" >/dev/null

echo "release forge-source manifest contract: OK"
