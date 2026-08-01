#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
script="$root/scripts/release/publish/publish-gitlab-release.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

if [ ! -f "$script" ]; then
  echo "missing release publisher script: $script" >&2
  exit 1
fi

version=0.1.0-test
artifacts="$tmp/dist"
remote="$tmp/remote"
mkdir "$artifacts" "$remote"

for name in \
  "aigw_${version}_darwin_amd64.tar.gz" \
  "aigw_${version}_darwin_arm64.tar.gz" \
  "aigw_${version}_darwin_universal.pkg" \
  "aigw_${version}_linux_amd64.deb" \
  "aigw_${version}_linux_amd64.rpm" \
  "aigw_${version}_linux_amd64.tar.gz" \
  "aigw_${version}_linux_arm64.deb" \
  "aigw_${version}_linux_arm64.rpm" \
  "aigw_${version}_linux_arm64.tar.gz" \
  "aigw_${version}_windows_amd64.msi" \
  "aigw_${version}_windows_amd64.zip" \
  "aigw_${version}_windows_arm64.msi" \
  "aigw_${version}_windows_arm64.zip" \
  "aigw_${version}.spdx.json"; do
  printf 'locally verified bytes for %s\n' "$name" > "$artifacts/$name"
done
(
  cd "$artifacts"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./* > checksums.txt
  else
    shasum -a 256 ./* > checksums.txt
  fi
)
cp "$artifacts"/* "$remote/"

cat > "$tmp/curl" <<'CURL'
#!/bin/sh
set -eu
method=GET
out=''
write=''
location=0
headers=''
job_token=absent
url=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --request|-X) method=$2; shift 2 ;;
    --output|-o) out=$2; shift 2 ;;
    --write-out|-w) write=$2; shift 2 ;;
    --location|-L) location=1; shift ;;
    --dump-header|-D) headers=$2; shift 2 ;;
    --header|-H)
      case "$2" in JOB-TOKEN:*) job_token=present ;; esac
      shift 2
      ;;
    --data|-d) shift 2 ;;
    --fail|--fail-with-body|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s %s\n' "$method" "$url" >> "$AIGW_TEST_CURL_LOG"
printf '%s token=%s location=%s\n' "$url" "$job_token" "$location" >> "$AIGW_TEST_HEADER_LOG"

response_mode=complete
case "$method" in
  POST) status=${AIGW_TEST_POST_STATUS:-201} ;;
  GET)
    case "$url" in
      "${AIGW_TEST_REDIRECT_URL:-not-configured}")
        status=200
        ;;
      */packages/generic/*)
        name=${url##*/}
        if [ "${AIGW_TEST_REDIRECT_ASSET:-}" = "$name" ] && [ "$location" -eq 1 ]; then
          printf '%s token=%s location=0\n' "$AIGW_TEST_REDIRECT_URL" "$job_token" >> "$AIGW_TEST_HEADER_LOG"
          status=200
        elif [ "${AIGW_TEST_REDIRECT_ASSET:-}" = "$name" ]; then
          status=302
        elif [ "${AIGW_TEST_MISSING_ASSET:-}" = "$name" ]; then
          status=404
        else
          status=200
        fi
        ;;
      *)
        release_get_count=$(grep -c '^GET .*/releases/' "$AIGW_TEST_CURL_LOG")
        if [ "$release_get_count" -eq 1 ]; then
          status=${AIGW_TEST_INITIAL_GET_STATUS:-404}
          response_mode=${AIGW_TEST_INITIAL_GET_MODE:-complete}
        else
          status=${AIGW_TEST_GET_STATUS:-200}
          response_mode=${AIGW_TEST_GET_MODE:-complete}
        fi
        ;;
    esac
    ;;
  *) status=500 ;;
esac

if [ -n "$headers" ]; then
  case "$status" in
    302) printf 'HTTP/1.1 302 Found\r\nLocation: %s\r\n\r\n' "$AIGW_TEST_REDIRECT_URL" > "$headers" ;;
    *) printf 'HTTP/1.1 %s Test\r\n\r\n' "$status" > "$headers" ;;
  esac
fi

if [ -n "$out" ]; then
  case "$method:$url:$status" in
    GET:*packages/generic/*:200|GET:https://objects.example.test/*:200)
      name=${url##*/}
      if [ "$out" != /dev/null ]; then
        cp "$AIGW_TEST_REMOTE/$name" "$out"
      fi
      ;;
    GET:*:2??)
      python3 - "$out" "$response_mode" <<'PY'
import json
import sys

out, mode = sys.argv[1:]
payload = json.load(open("release.json", encoding="utf-8"))
links = [{"url": item["url"]} for item in payload["assets"]["links"]]
tag = payload["tag_name"]
if mode == "missing-asset":
    links = links[:-1]
elif mode == "extra-asset":
    links.append({"url": links[0]["url"].rsplit("/", 1)[0] + "/unexpected.bin"})
elif mode == "duplicate-asset":
    links[-1] = {"url": links[0]["url"]}
elif mode == "wrong-tag":
    tag = "v9.9.9"
json.dump(
    {"tag_name": tag, "assets": {"links": links}},
    open(out, "w", encoding="utf-8"),
    separators=(",", ":"),
)
PY
      ;;
    *) printf '{"status":%s}\n' "$status" > "$out" ;;
  esac
fi
case "$write" in *http_code*) printf '%s' "$status" ;; esac
case "$status" in 2*|3*) exit 0 ;; *) exit 22 ;; esac
CURL
chmod 755 "$tmp/curl"

run_case() {
  name=$1
  initial_get=$2
  post=${3:-201}
  get_mode=${4:-complete}
  remote_directory=${5:-$remote}
  missing_asset=${6:-}
  artifact_directory=${7:-$artifacts}
  redirect_asset=${8:-}
  work="$tmp/$name"
  mkdir -p "$work"
  (
    cd "$work"
    set +e
    PATH="$tmp:$PATH" \
      AIGW_TEST_CURL_LOG="$work/curl.log" \
      AIGW_TEST_HEADER_LOG="$work/headers.log" \
      AIGW_TEST_INITIAL_GET_STATUS="$initial_get" \
      AIGW_TEST_POST_STATUS="$post" \
      AIGW_TEST_GET_MODE="$get_mode" \
      AIGW_TEST_REMOTE="$remote_directory" \
      AIGW_TEST_MISSING_ASSET="$missing_asset" \
      AIGW_TEST_REDIRECT_ASSET="$redirect_asset" \
      AIGW_TEST_REDIRECT_URL="https://objects.example.test/$redirect_asset" \
      CI_API_V4_URL=https://gitlab.example/api/v4 \
      CI_PROJECT_ID=456 \
      CI_COMMIT_TAG="v$version" \
      CI_JOB_TOKEN=redacted \
      sh "$script" "$artifact_directory"
    publish_status=$?
    set -e
    [ "$publish_status" -eq 0 ] || exit "$publish_status"
    python3 - <<'PY'
import json

payload = json.load(open("release.json", encoding="utf-8"))
assert payload["tag_name"] == "v0.1.0-test"
assert len(payload["assets"]["links"]) == 15
PY
  )
}

assert_no_overwrite_methods() {
  log=$1
  if grep -Ev '^(GET|POST) ' "$log" | grep -q .; then
    echo "release publisher issued an unexpected HTTP method" >&2
    exit 1
  fi
}

assert_get_only() {
  log=$1
  if grep -Ev '^GET ' "$log" | grep -q .; then
    echo "existing release verification issued a non-GET request" >&2
    exit 1
  fi
}

run_case created 404 201 >/dev/null
test "$(wc -l < "$tmp/created/curl.log" | tr -d ' ')" = 18
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/created/curl.log")" = 2
test "$(grep -c '^POST https://gitlab.example/api/v4/projects/456/releases$' "$tmp/created/curl.log")" = 1
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/created/curl.log")" = 15
assert_no_overwrite_methods "$tmp/created/curl.log"

run_case existing 200 500 >/dev/null
test "$(wc -l < "$tmp/existing/curl.log" | tr -d ' ')" = 17
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/existing/curl.log")" = 2
if grep -Ev '^GET ' "$tmp/existing/curl.log" | grep -q .; then
  echo "existing GitLab release verification issued a mutating HTTP request" >&2
  exit 1
fi

run_case concurrent-create 404 409 >/dev/null
test "$(wc -l < "$tmp/concurrent-create/curl.log" | tr -d ' ')" = 18
test "$(grep -c '^POST https://gitlab.example/api/v4/projects/456/releases$' "$tmp/concurrent-create/curl.log")" = 1
assert_no_overwrite_methods "$tmp/concurrent-create/curl.log"

if run_case missing-link 200 500 missing-asset >/dev/null 2>&1; then
  echo "release publisher accepted a release missing one of the 15 expected links" >&2
  exit 1
fi
assert_get_only "$tmp/missing-link/curl.log"

if run_case extra-link 200 500 extra-asset >/dev/null 2>&1; then
  echo "release publisher accepted an unexpected sixteenth release link" >&2
  exit 1
fi
assert_get_only "$tmp/extra-link/curl.log"

if run_case duplicate-link 200 500 duplicate-asset >/dev/null 2>&1; then
  echo "release publisher accepted duplicate release links" >&2
  exit 1
fi
assert_get_only "$tmp/duplicate-link/curl.log"

missing_name="aigw_${version}_linux_arm64.rpm"
if run_case missing-bytes 200 500 complete "$remote" "$missing_name" >/dev/null 2>&1; then
  echo "release publisher accepted a release with a missing remote asset" >&2
  exit 1
fi
assert_get_only "$tmp/missing-bytes/curl.log"

tampered_remote="$tmp/tampered-remote"
mkdir "$tampered_remote"
cp "$remote"/* "$tampered_remote/"
printf 'different remote bytes\n' > "$tampered_remote/aigw_${version}_linux_amd64.tar.gz"
if run_case mismatched-bytes 200 500 complete "$tampered_remote" >/dev/null 2>&1; then
  echo "release publisher accepted remote asset bytes that differ from the local matrix" >&2
  exit 1
fi
assert_get_only "$tmp/mismatched-bytes/curl.log"

self_consistent_tamper="$tmp/self-consistent-tamper"
mkdir "$self_consistent_tamper"
cp "$remote"/* "$self_consistent_tamper/"
printf 'different but self-consistent remote bytes\n' > "$self_consistent_tamper/aigw_${version}_windows_arm64.zip"
(
  cd "$self_consistent_tamper"
  rm checksums.txt
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./* > checksums.txt
  else
    shasum -a 256 ./* > checksums.txt
  fi
)
if run_case self-consistent-mismatch 200 500 complete "$self_consistent_tamper" >/dev/null 2>&1; then
  echo "release publisher trusted a self-consistent remote matrix instead of the local matrix" >&2
  exit 1
fi
assert_get_only "$tmp/self-consistent-mismatch/curl.log"

bad_manifest_remote="$tmp/bad-manifest-remote"
mkdir "$bad_manifest_remote"
cp "$remote"/* "$bad_manifest_remote/"
printf 'not a checksum manifest\n' > "$bad_manifest_remote/checksums.txt"
if run_case mismatched-manifest 200 500 complete "$bad_manifest_remote" >/dev/null 2>&1; then
  echo "release publisher accepted a remote checksums.txt that differs from the local manifest" >&2
  exit 1
fi
assert_get_only "$tmp/mismatched-manifest/curl.log"

redirect_name="aigw_${version}_darwin_arm64.tar.gz"
run_case cross-host-redirect 200 500 complete "$remote" '' "$artifacts" "$redirect_name" >/dev/null
grep -F "https://objects.example.test/$redirect_name token=absent" "$tmp/cross-host-redirect/headers.log" >/dev/null || {
  cat "$tmp/cross-host-redirect/headers.log" >&2
  echo "GitLab release verification forwarded JOB-TOKEN to a redirect host" >&2
  exit 1
}
assert_get_only "$tmp/cross-host-redirect/curl.log"

bad_local="$tmp/bad-local"
mkdir "$bad_local"
cp "$artifacts"/* "$bad_local/"
printf 'not a checksum manifest\n' > "$bad_local/checksums.txt"
if run_case malformed-local-manifest 200 500 complete "$remote" '' "$bad_local" >/dev/null 2>&1; then
  echo "release publisher contacted GitLab with an invalid local checksums.txt" >&2
  exit 1
fi
test ! -s "$tmp/malformed-local-manifest/curl.log"

mkdir -p "$tmp/denied"
if (
  cd "$tmp/denied"
  PATH="$tmp:$PATH" \
    AIGW_TEST_CURL_LOG="$tmp/denied/curl.log" \
    AIGW_TEST_HEADER_LOG="$tmp/denied/headers.log" \
    AIGW_TEST_INITIAL_GET_STATUS=500 \
    AIGW_TEST_REMOTE="$remote" \
    CI_API_V4_URL=https://gitlab.example/api/v4 \
    CI_PROJECT_ID=456 \
    CI_COMMIT_TAG="v$version" \
    CI_JOB_TOKEN=redacted \
    sh "$script" "$artifacts"
) >"$tmp/denied/output.log" 2>&1; then
  echo "release publisher accepted a failed preflight" >&2
  exit 1
fi
grep -Fq 'GitLab release preflight failed with HTTP 500' "$tmp/denied/output.log" || {
  cat "$tmp/denied/output.log" >&2
  echo "release publisher reported an unexpected failed-preflight diagnostic" >&2
  exit 1
}
test "$(wc -l < "$tmp/denied/curl.log" | tr -d ' ')" = 1
! grep -Eq '^(POST|PUT|PATCH|DELETE) ' "$tmp/denied/curl.log"

echo "immutable GitLab release publisher contract: OK"
