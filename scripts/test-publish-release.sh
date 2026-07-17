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
  PUT) status=${AIGW_TEST_PUT_STATUS:-200} ;;
  GET)
    case "${url:-}" in
      */packages/generic/*) status=${AIGW_TEST_ASSET_STATUS:-200} ;;
      *) status=${AIGW_TEST_GET_STATUS:-200} ;;
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
        *) python3 - "$out" "${AIGW_TEST_GET_MODE:-complete}" <<'PY'
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
  post=$2
  put=$3
  get_mode=${4:-complete}
  asset_status=${5:-200}
  work="$tmp/$name"
  mkdir -p "$work"
  (
    cd "$work"
    set +e
    PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$work/curl.log" AIGW_TEST_POST_STATUS="$post" AIGW_TEST_PUT_STATUS="$put" AIGW_TEST_GET_MODE="$get_mode" AIGW_TEST_ASSET_STATUS="$asset_status" AIGW_TEST_REQUIRE_LOCATION=1 \
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

run_case created 201 200
test "$(wc -l < "$tmp/created/curl.log" | tr -d ' ')" = 17
grep -q '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/created/curl.log"
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/created/curl.log")" = 15

aigw_409="$tmp/conflict"
run_case conflict 409 200
test "$(wc -l < "$aigw_409/curl.log" | tr -d ' ')" = 18
grep -q '^PUT ' "$aigw_409/curl.log"
grep -q '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$aigw_409/curl.log"
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$aigw_409/curl.log")" = 15

if run_case missing-asset 201 200 missing-asset; then
  echo "release publisher accepted a release missing one of the 15 expected assets" >&2
  exit 1
fi
test "$(wc -l < "$tmp/missing-asset/curl.log" | tr -d ' ')" = 16
grep -q '^GET https://gitlab.example/api/v4/projects/456/releases/v0.1.0-test$' "$tmp/missing-asset/curl.log"
test "$(grep -c '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/missing-asset/curl.log")" = 14

if run_case unavailable-asset 201 200 complete 404; then
  echo "release publisher accepted an unavailable linked Generic Package asset" >&2
  exit 1
fi
test "$(wc -l < "$tmp/unavailable-asset/curl.log" | tr -d ' ')" = 3
grep -q '^GET https://gitlab.example/api/v4/projects/456/packages/generic/aigw/0.1.0-test/' "$tmp/unavailable-asset/curl.log"

mkdir -p "$tmp/denied"
if (
  cd "$tmp/denied"
  PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$tmp/denied/curl.log" AIGW_TEST_POST_STATUS=403 AIGW_TEST_PUT_STATUS=200 \
    CI_API_V4_URL=https://gitlab.example/api/v4 CI_PROJECT_ID=456 CI_COMMIT_TAG=v0.1.0-test CI_JOB_TOKEN=redacted \
    sh "$script"
); then
  echo "release publisher accepted a non-conflict POST failure" >&2
  exit 1
fi
test "$(wc -l < "$tmp/denied/curl.log" | tr -d ' ')" = 1
! grep -q '^PUT ' "$tmp/denied/curl.log"

echo "idempotent release publisher contract: OK"
