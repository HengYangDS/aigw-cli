#!/bin/sh
set -eu

: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"

artifacts=${1:-dist}
[ -d "$artifacts" ] || { echo "artifact directory does not exist: $artifacts" >&2; exit 2; }
[ -s "$artifacts/checksums.txt" ] || { echo "missing checksums.txt" >&2; exit 2; }
for file in "$artifacts"/*; do
  [ -f "$file" ] || continue
  [ "$(basename "$file")" = checksums.txt ] && continue
  grep -E "^[0-9A-Fa-f]{64}  (\./)?$(basename "$file")$" "$artifacts/checksums.txt" >/dev/null || {
    echo "checksums.txt does not cover $(basename "$file")" >&2
    exit 2
  }
done

case "$GITHUB_REPOSITORY" in
  */*) ;;
  *) echo "GITHUB_REPOSITORY must be owner/repository" >&2; exit 2 ;;
esac
case "$CI_COMMIT_TAG" in
  v[0-9]*.*.*) ;;
  *) echo "CI_COMMIT_TAG must be a SemVer release tag" >&2; exit 2 ;;
esac
case "$GITHUB_TOKEN" in
  *"$(printf '\rX')"*|*"$(printf '\nX')"*) echo "GITHUB_TOKEN contains a control character" >&2; exit 2 ;;
esac

release=$(mktemp)
response=$(mktemp)
trap 'rm -f "$release" "$response"' EXIT HUP INT TERM
python3 - "$CI_COMMIT_TAG" > "$release" <<'PYTHON'
import json
import sys

tag = sys.argv[1]
print(json.dumps({
    "tag_name": tag,
    "name": "AIGW " + tag,
    "draft": True,
    "prerelease": "-" in tag,
    "generate_release_notes": True,
    "body": "Cross-platform AIGW CLI release. Verify every asset against checksums.txt.",
}))
PYTHON

api="https://api.github.com/repos/$GITHUB_REPOSITORY/releases"
status=$(curl --silent --show-error --output "$response" --write-out '%{http_code}' \
  --request POST --header "Authorization: Bearer $GITHUB_TOKEN" --header 'Accept: application/vnd.github+json' \
  --header 'Content-Type: application/json' --data @"$release" "$api" || true)
case "$status" in
  2??) upload_url=$(python3 - "$response" <<'PYTHON'
import json
import sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["upload_url"].split("{")[0])
PYTHON
) ;;
  422)
    status=$(curl --silent --show-error --output "$response" --write-out '%{http_code}' \
      --header "Authorization: Bearer $GITHUB_TOKEN" --header 'Accept: application/vnd.github+json' \
      "$api/tags/$CI_COMMIT_TAG" || true)
    case "$status" in
      2??) upload_url=$(python3 - "$response" <<'PYTHON'
import json
import sys
print(json.load(open(sys.argv[1], encoding="utf-8"))["upload_url"].split("{")[0])
PYTHON
) ;;
      *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release lookup failed with HTTP $status" >&2; exit 1 ;;
    esac
    ;;
  *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release publication failed with HTTP $status" >&2; exit 1 ;;
esac

for file in "$artifacts"/*; do
  [ -f "$file" ] || continue
  name=$(basename "$file")
  status=$(curl --silent --show-error --output "$response" --write-out '%{http_code}' \
    --request POST --header "Authorization: Bearer $GITHUB_TOKEN" --header 'Content-Type: application/octet-stream' \
    --data-binary @"$file" "$upload_url?name=$name" || true)
  case "$status" in
    2??) ;;
    422) ;;
    *) cat "$response" >&2 2>/dev/null || true; echo "GitHub asset upload failed for $name with HTTP $status" >&2; exit 1 ;;
  esac
done

# Releases are created draft-first. Publishing a draft is deliberately deferred
# to the protected release owner until remote parity has been inspected.
echo "GitHub release mirror draft: OK ($GITHUB_REPOSITORY $CI_COMMIT_TAG)"
