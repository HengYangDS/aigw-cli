#!/bin/sh
set -eu

artifacts=${1:-dist}
: "${CI_API_V4_URL:?CI_API_V4_URL is required}"
: "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"
: "${CI_JOB_TOKEN:?CI_JOB_TOKEN is required}"

version=${CI_COMMIT_TAG#v}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
"$root/scripts/check-release-artifacts.sh" "$artifacts" "$version"

base="$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/aigw/$version"

cat > release.json <<EOF
{
  "tag_name": "$CI_COMMIT_TAG",
  "name": "AIGW $CI_COMMIT_TAG",
  "description": "Cross-platform AIGW CLI release. Verify downloads with checksums.txt.",
  "assets": {"links": [
    {"name":"macOS Universal pkg","url":"$base/aigw_${version}_darwin_universal.pkg","direct_asset_path":"/aigw_${version}_darwin_universal.pkg"},
    {"name":"macOS amd64 portable","url":"$base/aigw_${version}_darwin_amd64.tar.gz","direct_asset_path":"/aigw_${version}_darwin_amd64.tar.gz"},
    {"name":"macOS arm64 portable","url":"$base/aigw_${version}_darwin_arm64.tar.gz","direct_asset_path":"/aigw_${version}_darwin_arm64.tar.gz"},
    {"name":"Linux amd64 deb","url":"$base/aigw_${version}_linux_amd64.deb","direct_asset_path":"/aigw_${version}_linux_amd64.deb"},
    {"name":"Linux arm64 deb","url":"$base/aigw_${version}_linux_arm64.deb","direct_asset_path":"/aigw_${version}_linux_arm64.deb"},
    {"name":"Linux amd64 rpm","url":"$base/aigw_${version}_linux_amd64.rpm","direct_asset_path":"/aigw_${version}_linux_amd64.rpm"},
    {"name":"Linux arm64 rpm","url":"$base/aigw_${version}_linux_arm64.rpm","direct_asset_path":"/aigw_${version}_linux_arm64.rpm"},
    {"name":"Linux amd64 portable","url":"$base/aigw_${version}_linux_amd64.tar.gz","direct_asset_path":"/aigw_${version}_linux_amd64.tar.gz"},
    {"name":"Linux arm64 portable","url":"$base/aigw_${version}_linux_arm64.tar.gz","direct_asset_path":"/aigw_${version}_linux_arm64.tar.gz"},
    {"name":"Windows amd64 msi","url":"$base/aigw_${version}_windows_amd64.msi","direct_asset_path":"/aigw_${version}_windows_amd64.msi"},
    {"name":"Windows arm64 msi","url":"$base/aigw_${version}_windows_arm64.msi","direct_asset_path":"/aigw_${version}_windows_arm64.msi"},
    {"name":"Windows amd64 portable","url":"$base/aigw_${version}_windows_amd64.zip","direct_asset_path":"/aigw_${version}_windows_amd64.zip"},
    {"name":"Windows arm64 portable","url":"$base/aigw_${version}_windows_arm64.zip","direct_asset_path":"/aigw_${version}_windows_arm64.zip"},
    {"name":"SHA-256 checksums","url":"$base/checksums.txt","direct_asset_path":"/checksums.txt"},
    {"name":"SPDX SBOM","url":"$base/aigw_${version}.spdx.json","direct_asset_path":"/aigw_${version}.spdx.json"}
  ]}
}
EOF

endpoint="$CI_API_V4_URL/projects/$CI_PROJECT_ID/releases"

download_release_asset() {
  asset_name=$1
  asset_url=$2
  output=$3
  current_url=$asset_url
  redirects=0
  response_body="$output.part"
  response_headers="$output.headers"

  while :; do
    send_token=$(python3 - "$CI_API_V4_URL" "$current_url" <<'PYTHON'
import sys
from urllib.parse import urlsplit

def authority(raw):
    parsed = urlsplit(raw)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise SystemExit(1)
    port = parsed.port or (443 if parsed.scheme == "https" else 80)
    return parsed.scheme.lower(), parsed.hostname.lower().rstrip("."), port

print("yes" if authority(sys.argv[1]) == authority(sys.argv[2]) else "no")
PYTHON
    ) || {
      echo "GitLab release verification found an invalid asset URL for $asset_name" >&2
      return 1
    }

    if [ "$send_token" = yes ]; then
      status=$(curl --silent --show-error --output "$response_body" --dump-header "$response_headers" --write-out '%{http_code}' \
        --header "JOB-TOKEN: $CI_JOB_TOKEN" "$current_url" || true)
    else
      status=$(curl --silent --show-error --output "$response_body" --dump-header "$response_headers" --write-out '%{http_code}' \
        "$current_url" || true)
    fi

    case "$status" in
      2??)
        mv "$response_body" "$output"
        rm -f "$response_headers"
        return 0
        ;;
      3??)
        redirects=$((redirects + 1))
        if [ "$redirects" -gt 5 ]; then
          echo "GitLab release verification exceeded the redirect limit for $asset_name" >&2
          return 1
        fi
        next_url=$(python3 - "$current_url" "$response_headers" <<'PYTHON'
import sys
from urllib.parse import urljoin, urlsplit

current, header_path = sys.argv[1:]
location = ""
with open(header_path, encoding="latin-1") as handle:
    for line in handle:
        name, separator, value = line.partition(":")
        if separator and name.strip().lower() == "location":
            location = value.strip()
if not location:
    raise SystemExit(1)
resolved = urljoin(current, location)
before = urlsplit(current)
after = urlsplit(resolved)
if after.scheme not in {"http", "https"} or not after.hostname:
    raise SystemExit(1)
if after.username is not None or after.password is not None or after.fragment:
    raise SystemExit(1)
if before.scheme == "https" and after.scheme != "https":
    raise SystemExit(1)
print(resolved)
PYTHON
        ) || {
          echo "GitLab release verification rejected an unsafe redirect for $asset_name" >&2
          rm -f "$response_body" "$response_headers"
          return 1
        }
        rm -f "$response_body" "$response_headers"
        current_url=$next_url
        ;;
      *)
        echo "GitLab release verification could not fetch asset $asset_name (HTTP $status)" >&2
        rm -f "$response_body" "$response_headers"
        return 1
        ;;
    esac
  done
}

verify_release() {
  status=$(curl --silent --show-error --output release-response.json --write-out '%{http_code}' \
    --header "JOB-TOKEN: $CI_JOB_TOKEN" "$endpoint/$CI_COMMIT_TAG" || true)
  case "$status" in
    2??) ;;
    *)
      cat release-response.json >&2 2>/dev/null || true
      echo "GitLab release verification failed with HTTP $status" >&2
      return 1
      ;;
  esac

  workspace=$(mktemp -d)
  asset_list=$(mktemp)
  trap 'rm -rf "$workspace"; rm -f "$asset_list"' EXIT HUP INT TERM

  python3 - release.json release-response.json "$asset_list" "$CI_COMMIT_TAG" <<'PYTHON'
import json
import sys

expected_path, actual_path, output_path, expected_tag = sys.argv[1:]

try:
    with open(expected_path, encoding="utf-8") as handle:
        expected_release = json.load(handle)
    with open(actual_path, encoding="utf-8") as handle:
        actual_release = json.load(handle)
except (OSError, ValueError) as exc:
    raise SystemExit(f"GitLab release verification returned invalid JSON: {exc}")

if actual_release.get("tag_name") != expected_tag:
    raise SystemExit("GitLab release verification returned the wrong tag")

try:
    expected_links = expected_release["assets"]["links"]
    actual_links = actual_release["assets"]["links"]
except (KeyError, TypeError):
    raise SystemExit("GitLab release verification returned an invalid asset manifest")

if not isinstance(expected_links, list) or len(expected_links) != 15:
    raise SystemExit(
        f"GitLab release verification expected 15 local asset links, found {len(expected_links) if isinstance(expected_links, list) else 0}"
    )
if not isinstance(actual_links, list) or len(actual_links) != 15:
    raise SystemExit(
        f"GitLab release verification expected 15 remote asset links, found {len(actual_links) if isinstance(actual_links, list) else 0}"
    )

expected_urls = []
downloads = []
for link in expected_links:
    if not isinstance(link, dict):
        raise SystemExit("GitLab release verification found an invalid local asset link")
    url = link.get("url")
    direct_path = link.get("direct_asset_path")
    if not isinstance(url, str) or not url:
        raise SystemExit("GitLab release verification found an invalid local asset URL")
    if not isinstance(direct_path, str) or not direct_path.startswith("/"):
        raise SystemExit("GitLab release verification found an invalid direct asset path")
    name = direct_path[1:]
    if not name or "/" in name or "\t" in name or "\n" in name:
        raise SystemExit("GitLab release verification found an unsafe direct asset path")
    expected_urls.append(url)
    downloads.append((name, url))

actual_urls = []
for link in actual_links:
    if not isinstance(link, dict) or not isinstance(link.get("url"), str):
        raise SystemExit("GitLab release verification found an invalid remote asset link")
    actual_urls.append(link["url"])

if len(set(expected_urls)) != 15:
    raise SystemExit("GitLab release verification found duplicate local asset URLs")
if len(set(actual_urls)) != 15:
    raise SystemExit("GitLab release verification found duplicate remote asset URLs")
if set(actual_urls) != set(expected_urls):
    missing = sorted(set(expected_urls) - set(actual_urls))
    extra = sorted(set(actual_urls) - set(expected_urls))
    if missing:
        raise SystemExit(f"GitLab release verification is missing asset {missing[0]}")
    raise SystemExit(f"GitLab release verification found unexpected asset {extra[0]}")

with open(output_path, "w", encoding="utf-8", newline="\n") as handle:
    for name, url in downloads:
        handle.write(f"{name}\t{url}\n")
PYTHON

  tab=$(printf '\t')
  while IFS="$tab" read -r asset_name asset_url; do
    if ! download_release_asset "$asset_name" "$asset_url" "$workspace/$asset_name"; then
      return 1
    fi
  done < "$asset_list"

  "$root/scripts/check-release-artifacts.sh" "$workspace" "$version" >/dev/null
  for local_asset in "$artifacts"/*; do
    [ -f "$local_asset" ] || continue
    asset_name=$(basename "$local_asset")
    if ! cmp -s "$local_asset" "$workspace/$asset_name"; then
      echo "GitLab release asset differs from locally verified $asset_name" >&2
      return 1
    fi
  done
}

status=$(curl --silent --show-error --output release-response.json --write-out '%{http_code}' \
  --header "JOB-TOKEN: $CI_JOB_TOKEN" "$endpoint/$CI_COMMIT_TAG" || true)
case "$status" in
  2??)
    ;;
  404)
    status=$(curl --silent --show-error --output release-response.json --write-out '%{http_code}' \
      --request POST --header "JOB-TOKEN: $CI_JOB_TOKEN" --header 'Content-Type: application/json' \
      --data @release.json "$endpoint" || true)
    case "$status" in
      2??|409) ;;
      *)
      cat release-response.json >&2 2>/dev/null || true
      echo "GitLab release publication failed with HTTP $status" >&2
      exit 1
      ;;
    esac
    ;;
  *)
    cat release-response.json >&2 2>/dev/null || true
    echo "GitLab release preflight failed with HTTP $status" >&2
    exit 1
    ;;
esac

verify_release
echo "GitLab release verified: $CI_COMMIT_TAG"
