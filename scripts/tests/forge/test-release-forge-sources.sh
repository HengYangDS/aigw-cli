#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

"$root/scripts/checks/forge/check-release-forge-sources.sh" >/dev/null
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

if AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  go run "$root/tools/releasekit" validate-release-sources >"$tmp/incomplete.out" 2>&1
then
  echo "release source validator accepted an incomplete GitHub tuple" >&2
  exit 1
fi
grep -F 'github release source is incomplete' "$tmp/incomplete.out" >/dev/null

if AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example/path \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  AIGW_GITHUB_RELEASE_REPOSITORY=organization/aigw-cli \
  go run "$root/tools/releasekit" validate-release-sources >"$tmp/invalid.out" 2>&1
then
  echo "release source validator accepted a non-origin URL" >&2
  exit 1
fi
grep -F 'release origin must be an HTTP(S) origin without credentials, path, query, or fragment' "$tmp/invalid.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/missing" >"$tmp/missing.out" 2>&1
then
  echo "package accepted missing release-source inputs" >&2
  exit 1
fi
grep -F 'release source is incomplete' "$tmp/missing.out" >/dev/null

if SOURCE_DATE_EPOCH=1784246400 \
  AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example \
  AIGW_GITLAB_RELEASE_REPOSITORY=group/aigw-cli \
  AIGW_GITHUB_RELEASE_ORIGIN=https://github.example \
  AIGW_GITHUB_RELEASE_REPOSITORY=organization/aigw-cli \
  sh "$root/scripts/release/build/package.sh" 0.1.0-release-source-test "$tmp/homepage" >"$tmp/homepage.out" 2>&1
then
  echo "package accepted a release without an explicit product homepage" >&2
  exit 1
fi
grep -Fx 'AIGW_PACKAGE_HOMEPAGE must be supplied by the release context' "$tmp/homepage.out" >/dev/null

echo "release forge-source execution contract: OK"
