#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/publish-release.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

if [ ! -f "$script" ]; then
  echo "missing release publisher script: $script" >&2
  exit 1
fi

cat > "$tmp/curl" <<'CURL'
#!/bin/sh
set -eu
method=GET
out=''
write=''
location=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --request|-X) method=$2; shift 2 ;;
    --output|-o) out=$2; shift 2 ;;
    --write-out|-w) write=$2; shift 2 ;;
    --location|-L) location=1; shift ;;
    --header|-H|--data|-d) shift 2 ;;
    --fail|--fail-with-body|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s %s\n' "$method" "${url:-}" >> "$AIGW_TEST_CURL_LOG"
case "$method" in
  POST) status=${AIGW_TEST_POST_STATUS:-201} ;;
  PUT|PATCH|DELETE) status=500 ;;
  GET)
    case "${url:-}" in
      */packages/generic/*) status=${AIGW_TEST_ASSET_STATUS:-200} ;;
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
if [ "${AIGW_TEST_REQUIRE_LOCATION:-0}" = 1 ]; then
  case "${url:-}" in
    */packages/generic/*)
      [ "$location" -eq 1 ] || { echo "asset fetch must follow redirects" >&2; exit 2; }
      ;;
  esac
fi
if [ -n "$out" ]; then
  case "$method" in
    GET)
      case "${url:-}" in
        */packages/generic/*) : ;;
        *) python3 - "$out" "${response_mode:-complete}" <<'PY'
import json
import sys

out, mode = sys.argv[1:]
payload = json.load(open("release.json"))
links = payload["assets"]["links"]
if mode == "missing-asset":
    links = links[:-1]
json.dump(
    {"tag_name": payload["tag_name"], "assets": {"links": [{"url": x["url"]} for x in links]}},
    open(out, "w"),
    separators=(",", ":"),
)
PY
        ;;
      esac
      ;;
    *) printf '{"status":%s}\n' "$status" > "$out" ;;
  esac
fi
case "$write" in *http_code*) printf '%s' "$status" ;; esac
case "$status" in 2*) exit 0;; *) exit 22;; esac
CURL
chmod 755 "$tmp/curl"

run_case() {
  name=$1
  initial_get=$2
  post=${3:-201}
  get_mode=${4:-complete}
  asset_status=${5:-200}
  work="$tmp/$name"
  mkdir -p "$work"
  (
    cd "$work"
    set +e
    PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$work/curl.log" AIGW_TEST_INITIAL_GET_STATUS="$initial_get" AIGW_TEST_POST_STATUS="$post" AIGW_TEST_GET_MODE="$get_mode" AIGW_TEST_ASSET_STATUS="$asset_status" AIGW_TEST_REQUIRE_LOCATION=1 \
      CI_API_V4_URL=https://gitlab.example/api/v4 CI_PROJECT_ID=456 CI_COMMIT_TAG=v0.1.0-test CI_JOB_TOKEN=redacted \
      sh "$script"
    publish_status=$?
    set -e
    [ "$publish_status" -eq 0 ] || exit "$publish_status"
    python3 - <<'PY'
import json
payload=json.load(open('release.json'))
assert payload['tag_name']=='v0.1.0-test'
assert len(payload['assets']['links'])==15
PY
  )
}

run_case created 404 201
test "$(wc -l < "$tmp/created/curl.log" | tr -d ' ')" = 18
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/created/curl.log")" = 2
test "$(grep -c '^POST https://gitlab.example/api/v4/projects/456/releases$' "$tmp/created/curl.log")" = 1
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/created/curl.log")" = 15
! grep -Eq '^(PUT|PATCH|DELETE) ' "$tmp/created/curl.log"

run_case existing 200 500
test "$(wc -l < "$tmp/existing/curl.log" | tr -d ' ')" = 17
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/existing/curl.log")" = 2
! grep -Eq '^(POST|PUT|PATCH|DELETE) ' "$tmp/existing/curl.log"

run_case concurrent-create 404 409
test "$(wc -l < "$tmp/concurrent-create/curl.log" | tr -d ' ')" = 18
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/concurrent-create/curl.log")" = 2
test "$(grep -c '^POST https://gitlab.example/api/v4/projects/456/releases$' "$tmp/concurrent-create/curl.log")" = 1
! grep -Eq '^(PUT|PATCH|DELETE) ' "$tmp/concurrent-create/curl.log"

if run_case missing-asset 404 201 missing-asset; then
  echo "release publisher accepted a release missing one of the 15 expected assets" >&2
  exit 1
fi
test "$(wc -l < "$tmp/missing-asset/curl.log" | tr -d ' ')" = 17
grep -q '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/missing-asset/curl.log"
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/missing-asset/curl.log")" = 14

if run_case unavailable-asset 404 201 complete 404; then
  echo "release publisher accepted an unavailable linked Generic Package asset" >&2
  exit 1
fi
test "$(wc -l < "$tmp/unavailable-asset/curl.log" | tr -d ' ')" = 4
grep -q '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/unavailable-asset/curl.log"

mkdir -p "$tmp/denied"
if (
  cd "$tmp/denied"
  PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$tmp/denied/curl.log" AIGW_TEST_INITIAL_GET_STATUS=500 AIGW_TEST_POST_STATUS=201 \
    CI_API_V4_URL=https://gitlab.example/api/v4 CI_PROJECT_ID=456 CI_COMMIT_TAG=v0.1.0-test CI_JOB_TOKEN=redacted \
    sh "$script"
); then
  echo "release publisher accepted a non-conflict POST failure" >&2
  exit 1
fi
test "$(wc -l < "$tmp/denied/curl.log" | tr -d ' ')" = 1
! grep -Eq '^(POST|PUT|PATCH|DELETE) ' "$tmp/denied/curl.log"

if run_case existing-missing-asset 200 500 missing-asset; then
  echo "release publisher accepted an incomplete existing release" >&2
  exit 1
fi
! grep -Eq '^(POST|PUT|PATCH|DELETE) ' "$tmp/existing-missing-asset/curl.log"

echo "immutable release publisher contract: OK"
