#!/bin/sh
set -eu

: "${CI_API_V4_URL:?CI_API_V4_URL is required}"
: "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"
: "${CI_JOB_TOKEN:?CI_JOB_TOKEN is required}"

version=${CI_COMMIT_TAG#v}
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

  if ! tr -d '[:space:]' < release-response.json | grep -Fq "\"tag_name\":\"$CI_COMMIT_TAG\""; then
    echo "GitLab release verification returned the wrong tag" >&2
    return 1
  fi

  asset_urls=$(sed -n 's/.*"url":"\([^"]*\)".*/\1/p' release.json)
  asset_count=0
  for asset_url in $asset_urls; do
    asset_count=$((asset_count + 1))
    if ! tr -d '[:space:]' < release-response.json | grep -Fq "\"url\":\"$asset_url\""; then
      echo "GitLab release verification is missing asset $asset_url" >&2
      return 1
    fi
    if ! curl --silent --show-error --fail --location --output /dev/null \
      --header "JOB-TOKEN: $CI_JOB_TOKEN" "$asset_url"; then
      echo "GitLab release verification could not fetch asset $asset_url" >&2
      return 1
    fi
  done
  if [ "$asset_count" -ne 15 ]; then
    echo "GitLab release verification expected 15 assets, found $asset_count in manifest" >&2
    return 1
  fi
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
