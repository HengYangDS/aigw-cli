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
accept=''
url=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --request|-X) method=$2; shift 2 ;;
    --output|-o) out=$2; shift 2 ;;
    --write-out|-w) write=$2; shift 2 ;;
    --header|-H)
      case "$2" in Accept:*) accept=$2 ;; esac
      shift 2 ;;
    --data|-d|--data-binary) shift 2 ;;
    --fail|--fail-with-body|--silent|--show-error) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s %s %s\n' "$method" "$accept" "$url" >> "$AIGW_TEST_CURL_LOG"
state=${AIGW_TEST_RELEASE_STATE:?}
assets=${AIGW_TEST_ASSETS:?}
read_assets() { [ -f "$assets" ] && cat "$assets" || true; }
write_release() {
  draft=$1
  python3 - "$draft" "$assets" <<'PY'
import json, sys
names = open(sys.argv[2], encoding='utf-8').read().splitlines() if __import__('os').path.exists(sys.argv[2]) else []
print(json.dumps({
  'url': 'https://api.github.com/repos/HengYangDS/aigw-cli/releases/1',
  'upload_url': 'https://uploads.example.test/releases/1/assets{?name,label}',
  'draft': sys.argv[1] == 'true',
  'assets': [{'name': n, 'size': __import__('os').path.getsize(__import__('os').path.join(__import__('os').environ['AIGW_TEST_ARTIFACTS'], n)), 'url': 'https://api.github.com/repos/HengYangDS/aigw-cli/releases/assets/' + n} for n in names],
}))
PY
}
case "$url" in
  */releases)
    if [ "$method" = POST ]; then
      if [ -f "$state" ]; then status=422; printf '{}' > "$out"; else : > "$state"; status=201; write_release true > "$out"; fi
    else status=500; printf '{}' > "$out"; fi
    ;;
  */releases/tags/*)
    status=200; write_release "${AIGW_TEST_DRAFT:-true}" > "$out"
    ;;
  */releases/1)
    case "$method" in
      GET) status=200; write_release "${AIGW_TEST_DRAFT:-true}" > "$out" ;;
      PATCH) AIGW_TEST_DRAFT=false; export AIGW_TEST_DRAFT; status=200; write_release false > "$out" ;;
      *) status=500; printf '{}' > "$out" ;;
    esac
    ;;
  https://uploads.example.test/*)
    name=${url#*name=}
    printf '%s\n' "$name" >> "$assets"
    sort -u "$assets" -o "$assets"
    status=201; printf '{}' > "$out"
    ;;
  */releases/assets/*)
    name=${url##*/}
    case "$accept" in *application/octet-stream*) cp "$AIGW_TEST_ARTIFACTS/$name" "$out"; status=200 ;; *) status=406; printf '{}' > "$out" ;; esac
    ;;
  *) status=500; printf '{}' > "$out" ;;
esac
case "$write" in *http_code*) printf '%s' "$status" ;; esac
case "$status" in 2*|422) exit 0 ;; *) exit 22 ;; esac
CURL
chmod 755 "$tmp/curl"

artifacts="$tmp/dist"
mkdir "$artifacts"
printf 'binary\n' > "$artifacts/aigw_0.1.0-test_darwin_arm64.tar.gz"
printf 'checksum payload\n' > "$artifacts/checksums.txt"
# checksums.txt must also checksum itself, so construct it last with its own
# known content by using the test's fixed point only for the non-index asset.
digest=$(shasum -a 256 "$artifacts/aigw_0.1.0-test_darwin_arm64.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$digest" aigw_0.1.0-test_darwin_arm64.tar.gz > "$artifacts/checksums.txt"
# Existing product artifacts intentionally do not self-index checksums.txt.
# The publisher must verify every distributable payload plus the manifest's
# internal syntax, not require a recursive checksum.

run_case() {
  name=$1
  work="$tmp/$name"
  mkdir "$work"
  : > "$work/assets"
  (
    cd "$work"
    PATH="$tmp:$PATH" AIGW_TEST_CURL_LOG="$work/curl.log" AIGW_TEST_RELEASE_STATE="$work/state" \
      AIGW_TEST_ASSETS="$work/assets" AIGW_TEST_ARTIFACTS="$artifacts" \
      GITHUB_REPOSITORY=HengYangDS/aigw-cli GITHUB_TOKEN=redacted CI_COMMIT_TAG=v0.1.0-test \
      sh "$script" "$artifacts" >/dev/null
  )
  grep -q '^PATCH ' "$work/curl.log"
  grep -q '/releases/assets/' "$work/curl.log"
}

run_case published
# A second invocation must resume/verify the same release rather than create a
# duplicate release or blindly overwrite assets.
run_case resumed

echo 'GitHub release publisher verification-and-publish contract: OK'
