#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/prepare-ci-go-cache.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

if [ ! -f "$script" ]; then
  echo "missing CI Go-cache preparation script: $script" >&2
  exit 1
fi

workspace="$tmp/workspace"
cache_base="$tmp/builds"
mkdir -p "$workspace" "$cache_base/.aigw-ci-cache/456/go-build" "$cache_base/.aigw-ci-cache/456/go-mod"
dd if=/dev/zero of="$cache_base/.aigw-ci-cache/456/go-mod/oversize" bs=1024 count=8 >/dev/null 2>&1

CI_BUILDS_DIR="$cache_base" CI_PROJECT_ID=456 CI_PROJECT_DIR="$workspace" AIGW_CI_CACHE_MAX_KIB=4 \
  sh -ceu '. "$1"; test "$AIGW_CI_CACHE_ROOT" = "$CI_BUILDS_DIR/.aigw-ci-cache/$CI_PROJECT_ID"; test "$GOCACHE" = "$AIGW_CI_CACHE_ROOT/go-build"; test "$GOMODCACHE" = "$AIGW_CI_CACHE_ROOT/go-mod"; case "$GOCACHE:$GOMODCACHE" in "$CI_PROJECT_DIR"/*) exit 1;; esac; case " $GOFLAGS " in *" -modcacherw "*) ;; *) exit 1;; esac; test ! -e "$GOMODCACHE/oversize"' sh "$script"

if CI_BUILDS_DIR="$cache_base" CI_PROJECT_ID='../escape' CI_PROJECT_DIR="$workspace" sh -ceu '. "$1"' sh "$script" >/dev/null 2>&1; then
  echo "CI cache preparation accepted an unsafe project ID" >&2
  exit 1
fi

echo "CI Go cache preparation contract: OK"
