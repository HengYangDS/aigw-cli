#!/bin/sh
set -eu

case "${CI_PROJECT_ID:-}" in
  ''|*[!0-9]*) echo "CI_PROJECT_ID must be numeric" >&2; return 2 2>/dev/null || exit 2 ;;
esac

cache_parent="${CI_BUILDS_DIR:-$HOME/builds}/.aigw-ci-cache"
AIGW_CI_CACHE_ROOT="$cache_parent/$CI_PROJECT_ID"
export AIGW_CI_CACHE_ROOT

cache_max_kib=${AIGW_CI_CACHE_MAX_KIB:-262144}
case "$cache_max_kib" in
  *[!0-9]*|'') echo "AIGW_CI_CACHE_MAX_KIB must be a positive integer" >&2; return 2 2>/dev/null || exit 2 ;;
esac

if [ -d "$AIGW_CI_CACHE_ROOT" ]; then
  size_kib=$(du -sk "$AIGW_CI_CACHE_ROOT" | awk '{print $1}')
  if [ "$size_kib" -gt "$cache_max_kib" ]; then
    rm -rf "$AIGW_CI_CACHE_ROOT"
  fi
fi

mkdir -p "$AIGW_CI_CACHE_ROOT/go-build" "$AIGW_CI_CACHE_ROOT/go-mod"
GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
GOCACHE="$AIGW_CI_CACHE_ROOT/go-build"
GOMODCACHE="$AIGW_CI_CACHE_ROOT/go-mod"
export GOFLAGS GOCACHE GOMODCACHE
