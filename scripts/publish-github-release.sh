#!/bin/sh
# Create, verify, and publish an exact GitHub Release from a signed local tag.
# This is a provider-native release publisher; it never depends on GitLab.
set -eu

: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"

artifacts=${1:-dist}
[ -d "$artifacts" ] || { echo "artifact directory does not exist: $artifacts" >&2; exit 2; }
[ -s "$artifacts/checksums.txt" ] || { echo "missing checksums.txt" >&2; exit 2; }
case "$GITHUB_REPOSITORY" in */*) ;; *) echo "GITHUB_REPOSITORY must be owner/repository" >&2; exit 2 ;; esac
case "$CI_COMMIT_TAG" in v[0-9]*.*.*) ;; *) echo "CI_COMMIT_TAG must be a SemVer release tag" >&2; exit 2 ;; esac
case "$GITHUB_TOKEN" in *"$(printf '\rX')"*|*"$(printf '\nX')"*) echo "GITHUB_TOKEN contains a control character" >&2; exit 2 ;; esac

manifest_entry() {
  name=$1
  matches=$(awk -v name="$name" '$2 == name || $2 == "./" name { count++; digest=$1 } END { if (count == 1 && digest ~ /^[0-9A-Fa-f]{64}$/) print digest; else exit 1 }' "$artifacts/checksums.txt") || return 1
  printf '%s\n' "$matches"
}

asset_names="$artifacts/.github-release-assets"
: > "$asset_names"
for file in "$artifacts"/*; do
  [ -f "$file" ] || continue
  name=$(basename "$file")
  # checksums.txt indexes distributable payloads; a self-entry would be a
  # recursive checksum with no stable fixed point. The manifest itself is
  # uploaded and byte-compared during the remote verification phase.
  if [ "$name" != checksums.txt ]; then
    actual=$(shasum -a 256 "$file" | awk '{print $1}')
    expected=$(manifest_entry "$name") || {
      echo "checksums.txt must contain exactly one SHA-256 entry for $name" >&2
      exit 2
    }
    [ "$actual" = "$expected" ] || {
      echo "checksum mismatch before GitHub publication: $name" >&2
      exit 2
    }
  fi
  printf '%s\n' "$name" >> "$asset_names"
done
sort -u "$asset_names" -o "$asset_names"
[ "$(wc -l < "$asset_names" | tr -d ' ')" -ge 2 ] || {
  echo "release artifact set must contain checksums.txt and at least one asset" >&2
  exit 2
}

response=$(mktemp)
release_json=$(mktemp)
trap 'rm -f "$response" "$release_json" "$asset_names"' EXIT HUP INT TERM
api="https://api.github.com/repos/$GITHUB_REPOSITORY/releases"
request_json() {
  method=$1
  url=$2
  body=${3:-}
  if [ -n "$body" ]; then
    curl --silent --show-error --output "$response" --write-out '%{http_code}' \
      --request "$method" --header "Authorization: Bearer $GITHUB_TOKEN" \
      --header 'Accept: application/vnd.github+json' --header 'Content-Type: application/json' \
      --data @"$body" "$url" || true
  else
    curl --silent --show-error --output "$response" --write-out '%{http_code}' \
      --request "$method" --header "Authorization: Bearer $GITHUB_TOKEN" \
      --header 'Accept: application/vnd.github+json' "$url" || true
  fi
}

python3 - "$CI_COMMIT_TAG" > "$release_json" <<'PYTHON'
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

status=$(request_json POST "$api" "$release_json")
case "$status" in
  2??) ;;
  422)
    status=$(request_json GET "$api/tags/$CI_COMMIT_TAG")
    case "$status" in 2??) ;; *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release lookup failed with HTTP $status" >&2; exit 1 ;; esac
    ;;
  *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release creation failed with HTTP $status" >&2; exit 1 ;;
esac

read_release() {
  python3 - "$response" "$asset_names" <<'PYTHON'
import json
import sys

release = json.load(open(sys.argv[1], encoding="utf-8"))
expected = set(open(sys.argv[2], encoding="utf-8").read().splitlines())
assets = {item.get("name"): item for item in release.get("assets", []) if item.get("name")}
print(release["url"])
print(release["upload_url"].split("{")[0])
print("true" if release.get("draft", False) else "false")
print(" ".join(sorted(assets)))
print(" ".join(sorted(expected)))
PYTHON
}
release_state=$(read_release)
release_url=$(printf '%s\n' "$release_state" | sed -n '1p')
upload_url=$(printf '%s\n' "$release_state" | sed -n '2p')
is_draft=$(printf '%s\n' "$release_state" | sed -n '3p')
remote_assets=$(printf '%s\n' "$release_state" | sed -n '4p')
expected_assets=$(printf '%s\n' "$release_state" | sed -n '5p')
[ "$is_draft" = true ] || {
  [ "$remote_assets" = "$expected_assets" ] || { echo "published GitHub release assets diverge from local matrix" >&2; exit 1; }
  echo "GitHub release already published and checksum matrix matches: OK ($GITHUB_REPOSITORY $CI_COMMIT_TAG)"
  exit 0
}

for name in $(cat "$asset_names"); do
  case " $remote_assets " in
    *" $name "*) continue ;;
  esac
  status=$(curl --silent --show-error --output "$response" --write-out '%{http_code}' \
    --request POST --header "Authorization: Bearer $GITHUB_TOKEN" \
    --header 'Content-Type: application/octet-stream' --data-binary @"$artifacts/$name" \
    "$upload_url?name=$name" || true)
  case "$status" in
    2??) ;;
    *) cat "$response" >&2 2>/dev/null || true; echo "GitHub asset upload failed for $name with HTTP $status" >&2; exit 1 ;;
  esac
done

status=$(request_json GET "$release_url")
case "$status" in 2??) ;; *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release verification failed with HTTP $status" >&2; exit 1 ;; esac
python3 - "$response" "$artifacts" "$asset_names" <<'PYTHON'
import hashlib
import json
import sys

release = json.load(open(sys.argv[1], encoding="utf-8"))
root, expected_path = sys.argv[2:]
expected = set(open(expected_path, encoding="utf-8").read().splitlines())
assets = {item.get("name"): item for item in release.get("assets", []) if item.get("name")}
if set(assets) != expected:
    raise SystemExit(f"remote asset set mismatch: got {sorted(assets)}, expected {sorted(expected)}")
# GitHub reports size, not content digests. Download every asset and compare
# to the local verified bytes before publishing the draft.
for name in sorted(expected):
    if assets[name].get("size") != __import__("os").path.getsize(__import__("os").path.join(root, name)):
        raise SystemExit(f"remote asset size mismatch: {name}")
PYTHON
# Exact bytes are checked through the download API to prevent a same-size
# replacement. Download one asset at a time with authentication and compare.
for name in $(cat "$asset_names"); do
  asset_url=$(python3 - "$response" "$name" <<'PYTHON'
import json
import sys
for asset in json.load(open(sys.argv[1], encoding="utf-8")).get("assets", []):
    if asset.get("name") == sys.argv[2]:
        print(asset["url"])
        break
else:
    raise SystemExit(1)
PYTHON
)
  downloaded=$(mktemp)
  trap 'rm -f "$response" "$release_json" "$asset_names" "$downloaded"' EXIT HUP INT TERM
  status=$(curl --silent --show-error --output "$downloaded" --write-out '%{http_code}' \
    --header "Authorization: Bearer $GITHUB_TOKEN" --header 'Accept: application/octet-stream' "$asset_url" || true)
  case "$status" in 2??) ;; *) rm -f "$downloaded"; echo "GitHub asset verification download failed for $name with HTTP $status" >&2; exit 1 ;; esac
  cmp -s "$downloaded" "$artifacts/$name" || { rm -f "$downloaded"; echo "GitHub remote asset bytes mismatch: $name" >&2; exit 1; }
  rm -f "$downloaded"
done

python3 - "$CI_COMMIT_TAG" > "$release_json" <<'PYTHON'
import json
import sys
print(json.dumps({"tag_name": sys.argv[1], "draft": False}))
PYTHON
status=$(request_json PATCH "$release_url" "$release_json")
case "$status" in 2??) ;; *) cat "$response" >&2 2>/dev/null || true; echo "GitHub release publication failed with HTTP $status" >&2; exit 1 ;; esac
printf 'GitHub release published with verified assets: OK (%s %s)\n' "$GITHUB_REPOSITORY" "$CI_COMMIT_TAG"
