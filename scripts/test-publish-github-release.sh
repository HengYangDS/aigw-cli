#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/publish-github-release.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

cat > "$tmp/curl" <<'CURL'
#!/bin/sh
set -eu
method=GET
out=''
write=''
url=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --request|-X) method=$2; shift 2 ;;
    --output|-o) out=$2; shift 2 ;;
    --write-out|-w) write=$2; shift 2 ;;
    --header|-H|--data|-d) shift 2 ;;
    --data-binary) shift 2 ;;
    --fail|--fail-with-body|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s %s\n' "$method" "$url" >> "$AIGW_TEST_CURL_LOG"
case "$url" in
  */releases)
    status=${AIGW_TEST_CREATE_STATUS:-201}
    payload='{"upload_url":"https://uploads.example.test/releases/1/assets{?name,label}"}'
    ;;
  */releases/tags/*)
    status=${AIGW_TEST_LOOKUP_STATUS:-200}
    payload='{"upload_url":"https://uploads.example.test/releases/1/assets{?name,label}"}'
    ;;
  https://uploads.example.test/*)
    status=${AIGW_TEST_UPLOAD_STATUS:-201}
    payload='{}'
    ;;
  *) status=500; payload='{}' ;;
esac
[ -n "$out" ] && printf '%s\n' "$payload" > "$out"
case "$write" in *http_code*) printf '%s' "$status" ;; esac
case "$status" in 2*|422) exit 0 ;; *) exit 22 ;; esac
CURL
chmod 755 "$tmp/curl"

artifacts="$tmp/dist"
mkdir "$artifacts"
printf 'binary\n' > "$artifacts/aigw_0.1.0-test_darwin_arm64.tar.gz"
digest=$(shasum -a 256 "$artifacts/aigw_0.1.0-test_darwin_arm64.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$digest" aigw_0.1.0-test_darwin_arm64.tar.gz > "$artifacts/checksums.txt"

run_case() {
  name=$1
  create=$2
  work="$tmp/$name"
  mkdir "$work"
  (
    cd "$work"
    PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$work/curl.log" AIGW_TEST_CREATE_STATUS="$create" \
      GITHUB_REPOSITORY=HengYangDS/aigw-cli GITHUB_TOKEN=redacted CI_COMMIT_TAG=v0.1.0-test \
      sh "$script" "$artifacts" >/dev/null
  )
}

run_case created 201
test "$(wc -l < "$tmp/created/curl.log" | tr -d ' ')" = 3
run_case existing 422
grep -q '/releases/tags/v0.1.0-test' "$tmp/existing/curl.log"

cp "$artifacts/checksums.txt" "$artifacts/checksums.good"
printf '0  aigw_0.1.0-test_darwin_arm64.tar.gz\n' > "$artifacts/checksums.txt"
if (
  PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$tmp/unverified.log" GITHUB_REPOSITORY=HengYangDS/aigw-cli GITHUB_TOKEN=redacted CI_COMMIT_TAG=v0.1.0-test \
    sh "$script" "$artifacts"
); then
  echo "GitHub publisher accepted an unverified artifact set" >&2
  exit 1
fi
[ ! -e "$tmp/unverified.log" ] || { echo "GitHub publisher contacted the API before checksum admission" >&2; exit 1; }
mv "$artifacts/checksums.good" "$artifacts/checksums.txt"

if (
  PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$tmp/invalid.log" GITHUB_REPOSITORY=bad GITHUB_TOKEN=redacted CI_COMMIT_TAG=v0.1.0-test \
    sh "$script" "$artifacts"
); then
  echo "GitHub publisher accepted invalid repository identity" >&2
  exit 1
fi

echo "idempotent GitHub mirror publisher contract: OK"
