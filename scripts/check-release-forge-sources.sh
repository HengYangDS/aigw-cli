#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
manifest=${AIGW_RELEASE_FORGE_SOURCES_FILE:-$root/packaging/release/forge-sources.env}

[ -f "$manifest" ] || {
  echo "release forge-source manifest is missing: $manifest" >&2
  exit 1
}

keys='
AIGW_GITLAB_RELEASE_ORIGIN
AIGW_GITLAB_RELEASE_REPOSITORY
AIGW_GITHUB_RELEASE_ORIGIN
AIGW_GITHUB_RELEASE_REPOSITORY
'

for key in $keys; do
  count=$(grep -Ec "^${key}=" "$manifest" || true)
  [ "$count" -eq 1 ] || {
    echo "release forge-source manifest must define $key exactly once" >&2
    exit 1
  }
done

if grep -Ev '^(#.*|[A-Z0-9_]+=[^[:space:]#]+)$' "$manifest" | grep -q .; then
  echo "release forge-source manifest contains an invalid line" >&2
  exit 1
fi

# shellcheck source=../packaging/release/forge-sources.env
. "$manifest"

case "$AIGW_GITLAB_RELEASE_ORIGIN" in http://*|https://*) ;; *) echo "GitLab release origin must be an absolute HTTP(S) URL" >&2; exit 1 ;; esac
case "$AIGW_GITHUB_RELEASE_ORIGIN" in https://*) ;; *) echo "GitHub release origin must be an absolute HTTPS URL" >&2; exit 1 ;; esac
case "$AIGW_GITLAB_RELEASE_REPOSITORY" in */*) ;; *) echo "GitLab release repository must be a project path" >&2; exit 1 ;; esac
case "$AIGW_GITHUB_RELEASE_REPOSITORY" in */*) ;; *) echo "GitHub release repository must be an owner/repository path" >&2; exit 1 ;; esac

echo "release forge-source manifest: OK"
