#!/bin/sh
set -eu

artifacts=${1:-dist}
: "${CI_API_V4_URL:?CI_API_V4_URL is required}"
: "${CI_PROJECT_ID:?CI_PROJECT_ID is required}"
: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required}"
: "${CI_JOB_TOKEN:?CI_JOB_TOKEN is required}"

version=${CI_COMMIT_TAG#v}
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
"$root/scripts/checks/release/check-release-artifacts.sh" "$artifacts" "$version"
workspace_root=$(pwd -P)
release_json="$workspace_root/release.json"
release_response="$workspace_root/release-response.json"

base="$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/aigw/$version"

(cd "$root" && go run -buildvcs=false ./tools/releasekit write-gitlab-release "$CI_COMMIT_TAG" "$base" "$release_json")

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
    send_token=$(cd "$root" && go run -buildvcs=false ./tools/releasekit same-authority "$CI_API_V4_URL" "$current_url") || {
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
        next_url=$(cd "$root" && go run -buildvcs=false ./tools/releasekit resolve-redirect "$current_url" "$response_headers") || {
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
  status=$(curl --silent --show-error --output "$release_response" --write-out '%{http_code}' \
    --header "JOB-TOKEN: $CI_JOB_TOKEN" "$endpoint/$CI_COMMIT_TAG" || true)
  case "$status" in
    2??) ;;
    *)
      cat "$release_response" >&2 2>/dev/null || true
      echo "GitLab release verification failed with HTTP $status" >&2
      return 1
      ;;
  esac

  workspace=$(mktemp -d)
  asset_list=$(mktemp)
  trap 'rm -rf "$workspace"; rm -f "$asset_list"' EXIT HUP INT TERM

  (cd "$root" && go run -buildvcs=false ./tools/releasekit verify-gitlab-release \
    "$release_json" "$release_response" "$asset_list" "$CI_COMMIT_TAG")

  tab=$(printf '\t')
  while IFS="$tab" read -r asset_name asset_url; do
    if ! download_release_asset "$asset_name" "$asset_url" "$workspace/$asset_name"; then
      return 1
    fi
  done < "$asset_list"

  "$root/scripts/checks/release/check-release-artifacts.sh" "$workspace" "$version" >/dev/null
  for local_asset in "$artifacts"/*; do
    [ -f "$local_asset" ] || continue
    asset_name=$(basename "$local_asset")
    if ! cmp -s "$local_asset" "$workspace/$asset_name"; then
      echo "GitLab release asset differs from locally verified $asset_name" >&2
      return 1
    fi
  done
}

status=$(curl --silent --show-error --output "$release_response" --write-out '%{http_code}' \
  --header "JOB-TOKEN: $CI_JOB_TOKEN" "$endpoint/$CI_COMMIT_TAG" || true)
case "$status" in
  2??)
    ;;
  404)
    status=$(curl --silent --show-error --output "$release_response" --write-out '%{http_code}' \
      --request POST --header "JOB-TOKEN: $CI_JOB_TOKEN" --header 'Content-Type: application/json' \
      --data @"$release_json" "$endpoint" || true)
    case "$status" in
      2??|409) ;;
      *)
      cat "$release_response" >&2 2>/dev/null || true
      echo "GitLab release publication failed with HTTP $status" >&2
      exit 1
      ;;
    esac
    ;;
  *)
    cat "$release_response" >&2 2>/dev/null || true
    echo "GitLab release preflight failed with HTTP $status" >&2
    exit 1
    ;;
esac

verify_release
echo "GitLab release verified: $CI_COMMIT_TAG"
