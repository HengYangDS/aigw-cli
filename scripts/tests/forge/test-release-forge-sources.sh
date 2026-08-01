#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
manifest="$root/.config/release/forge-sources.env"
resolver="$root/scripts/release/lib/resolve-release-forge-sources.py"

"$root/scripts/checks/forge/check-release-forge-sources.sh" >/dev/null
go_version=$(awk '$1 == "go" { print $2; exit }' "$root/go.mod")
[ -n "$go_version" ] || { echo "go.mod has no Go version" >&2; exit 1; }
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

python3 "$resolver" > "$tmp/default.env"
python3 - "$tmp/default.env" <<'PY'
import sys

values = {}
for line in open(sys.argv[1], encoding="utf-8"):
    key, value = line.rstrip("\n").split("=", 1)
    values[key] = value
expected = {
    "AIGW_GITLAB_RELEASE_ORIGIN",
    "AIGW_GITLAB_RELEASE_REPOSITORY",
    "AIGW_GITHUB_RELEASE_ORIGIN",
    "AIGW_GITHUB_RELEASE_REPOSITORY",
}
if set(values) != expected or not all(values.values()):
    raise SystemExit("resolver did not emit one complete tuple for each forge")
PY

shell=$(python3 "$resolver" --shell)
case "$shell" in
  *'export AIGW_GITLAB_RELEASE_ORIGIN='*'export AIGW_GITHUB_RELEASE_REPOSITORY='*) ;;
  *) echo "resolver did not emit all POSIX exports" >&2; exit 1 ;;
esac

cp "$manifest" "$tmp/manifest"
printf 'AIGW_GITHUB_RELEASE_ORIGIN=https://duplicate.example.test\n' >> "$tmp/manifest"
if AIGW_RELEASE_FORGE_SOURCES_FILE="$tmp/manifest" python3 "$resolver" > "$tmp/duplicate.out" 2>&1; then
  echo "forge-source checker accepted a duplicate key" >&2
  exit 1
fi
grep -F 'must define AIGW_GITHUB_RELEASE_ORIGIN exactly once' "$tmp/duplicate.out" >/dev/null

cp "$manifest" "$tmp/manifest"
python3 - "$tmp/manifest" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
path.write_text(
    path.read_text(encoding="utf-8").replace(
        "AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example",
        "AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example.test/path",
    ),
    encoding="utf-8",
)
PY
if AIGW_RELEASE_FORGE_SOURCES_FILE="$tmp/manifest" python3 "$resolver" > "$tmp/invalid.out" 2>&1; then
  echo "forge-source resolver accepted a non-origin URL" >&2
  exit 1
fi
grep -F 'AIGW_GITLAB_RELEASE_ORIGIN must not include credentials, path, query, or fragment' "$tmp/invalid.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/conflict" > "$tmp/conflict.out" 2>&1; then
  echo "package accepted missing release-source inputs" >&2
  exit 1
fi
grep -F 'execution environment must define AIGW_GITLAB_RELEASE_ORIGIN' "$tmp/conflict.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  AIGW_GITHUB_RELEASE_REPOSITORY=organization/aigw-cli \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/homepage" > "$tmp/homepage.out" 2>&1; then
  echo "package accepted a release without an explicit product homepage" >&2
  exit 1
fi
grep -Fx 'AIGW_PACKAGE_HOMEPAGE must be supplied by the release context' "$tmp/homepage.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  AIGW_GITHUB_RELEASE_REPOSITORY=organization/aigw-cli \
  AIGW_PACKAGE_HOMEPAGE=http://product.example/aigw-cli \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/insecure-homepage" > "$tmp/insecure-homepage.out" 2>&1; then
  echo "package accepted a non-HTTPS product homepage" >&2
  exit 1
fi
grep -Fx 'AIGW_PACKAGE_HOMEPAGE must be an https URL' "$tmp/insecure-homepage.out" >/dev/null

printf 'not a release manifest\n' > "$tmp/foreign-manifest"
if SOURCE_DATE_EPOCH=1784246400 \
  AIGW_RELEASE_FORGE_SOURCES_FILE="$tmp/foreign-manifest" \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/foreign" > "$tmp/foreign.out" 2>&1; then
  echo "package accepted a manifest in place of explicit execution inputs" >&2
  exit 1
fi
grep -F 'execution environment must define AIGW_GITLAB_RELEASE_ORIGIN' "$tmp/foreign.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  AIGW_RELEASE_GO_TOOLCHAIN=go0.0.0 \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/toolchain" > "$tmp/toolchain.out" 2>&1; then
  echo "package accepted a conflicting release toolchain override" >&2
  exit 1
fi
grep -Fx "AIGW_RELEASE_GO_TOOLCHAIN conflicts with go.mod: expected go$go_version" "$tmp/toolchain.out" >/dev/null

echo "release forge-source execution contract: OK"
