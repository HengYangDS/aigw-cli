#!/bin/sh
# Publish a complete GitHub release once, or verify the existing release is
# byte-for-byte identical. It never overwrites a release asset.
set -eu

artifacts=${1:?usage: publish-github-release.sh <artifact-directory>}
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"

case "$GITHUB_REPOSITORY" in
  */*) ;;
  *) echo "GITHUB_REPOSITORY must be an owner/repository path" >&2; exit 2 ;;
esac
case "$CI_COMMIT_TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "CI_COMMIT_TAG must be a v<semver> tag" >&2; exit 2 ;;
esac

version=${CI_COMMIT_TAG#v}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
"$root/scripts/check-release-artifacts.sh" "$artifacts" "$version"

token=${GH_TOKEN:-${GITHUB_TOKEN:-}}
[ -n "$token" ] || { echo "GH_TOKEN or GITHUB_TOKEN is required" >&2; exit 2; }
export GH_TOKEN=$token

if ! gh release view "$CI_COMMIT_TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
  gh release create "$CI_COMMIT_TAG" --repo "$GITHUB_REPOSITORY" --verify-tag \
    --title "AIGW $CI_COMMIT_TAG" --generate-notes "$artifacts"/*
  echo "GitHub release created: $GITHUB_REPOSITORY $CI_COMMIT_TAG"
  exit 0
fi

workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM
for file in "$artifacts"/*; do
  [ -f "$file" ] || continue
  name=$(basename "$file")
  gh release download "$CI_COMMIT_TAG" --repo "$GITHUB_REPOSITORY" \
    --pattern "$name" --dir "$workspace"
done
"$root/scripts/check-release-artifacts.sh" "$workspace" "$version"

for file in "$artifacts"/*; do
  [ -f "$file" ] || continue
  name=$(basename "$file")
  if command -v sha256sum >/dev/null 2>&1; then
    local_digest=$(sha256sum "$file" | awk '{print tolower($1)}')
    remote_digest=$(sha256sum "$workspace/$name" | awk '{print tolower($1)}')
  else
    local_digest=$(shasum -a 256 "$file" | awk '{print tolower($1)}')
    remote_digest=$(shasum -a 256 "$workspace/$name" | awk '{print tolower($1)}')
  fi
  [ "$local_digest" = "$remote_digest" ] || {
    echo "GitHub release asset differs from locally verified $name" >&2
    exit 1
  }
done

echo "GitHub release already matches: $GITHUB_REPOSITORY $CI_COMMIT_TAG"
