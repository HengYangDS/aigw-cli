#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
version=${1:?usage: release-source-date-epoch.sh <version> [changelog]}
changelog=${2:-"$root/CHANGELOG.md"}

[ -f "$changelog" ] || {
  echo "changelog does not exist: $changelog" >&2
  exit 2
}

repositorycheck=${AIGW_REPOSITORY_CHECK:-}
if [ -z "$repositorycheck" ]; then
  repositorycheck=$(mktemp "${TMPDIR:-/tmp}/aigw-repositorycheck.XXXXXX")
  trap 'rm -f "$repositorycheck"' EXIT HUP INT TERM
  (cd "$root" && go build -o "$repositorycheck" ./tools/repositorycheck)
fi
"$repositorycheck" --root "$root" release-epoch "$version" "$changelog"
