#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

changelog=${AIGW_CHANGELOG_FILE:-CHANGELOG.md}
case "$changelog" in
  /*) ;;
  *) changelog="$root/$changelog" ;;
esac

selected_tag=${AIGW_CHANGELOG_RELEASE_TAG:-${CI_COMMIT_TAG:-}}
if test -z "$selected_tag" && test "${GITHUB_REF_TYPE:-}" = tag; then
  selected_tag=${GITHUB_REF_NAME:-}
fi

repositorycheck=${AIGW_REPOSITORY_CHECK:-}
if [ -z "$repositorycheck" ]; then
  repositorycheck=$(mktemp "${TMPDIR:-/tmp}/aigw-repositorycheck.XXXXXX")
  trap 'rm -f "$repositorycheck"' EXIT HUP INT TERM
  go build -o "$repositorycheck" ./tools/repositorycheck
fi
"$repositorycheck" --root "$root" changelog "$changelog" "$selected_tag"
