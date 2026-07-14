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
while [ "$#" -gt 0 ]; do
  case "$1" in
    --request|-X) method=$2; shift 2 ;;
    --output|-o) out=$2; shift 2 ;;
    --write-out|-w) write=$2; shift 2 ;;
    --header|-H|--data|-d) shift 2 ;;
    --fail|--fail-with-body|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s %s\n' "$method" "${url:-}" >> "$AIGW_TEST_CURL_LOG"
case "$method" in
  POST) status=${AIGW_TEST_POST_STATUS:-201} ;;
  PUT) status=${AIGW_TEST_PUT_STATUS:-200} ;;
  *) status=500 ;;
esac
[ -n "$out" ] && printf '{"status":%s}\n' "$status" > "$out"
case "$write" in *http_code*) printf '%s' "$status" ;; esac
case "$status" in 2*) exit 0;; *) exit 22;; esac
CURL
chmod 755 "$tmp/curl"

run_case() {
  name=$1
  post=$2
  put=$3
  work="$tmp/$name"
  mkdir -p "$work"
  (
    cd "$work"
    PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$work/curl.log" AIGW_TEST_POST_STATUS="$post" AIGW_TEST_PUT_STATUS="$put" \
      CI_API_V4_URL=https://gitlab.example/api/v4 CI_PROJECT_ID=456 CI_COMMIT_TAG=v0.1.0-test CI_JOB_TOKEN=redacted \
      sh "$script"
    python3 - <<'PY'
import json
payload=json.load(open('release.json'))
assert payload['tag_name']=='v0.1.0-test'
assert len(payload['assets']['links'])==15
PY
  )
}

run_case created 201 200
test "$(wc -l < "$tmp/created/curl.log" | tr -d ' ')" = 1

aigw_409="$tmp/conflict"
run_case conflict 409 200
test "$(wc -l < "$aigw_409/curl.log" | tr -d ' ')" = 2
grep -q '^PUT ' "$aigw_409/curl.log"

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
