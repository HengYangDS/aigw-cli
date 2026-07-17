#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
script="$root/scripts/publish-github-release.sh"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

[ -x "$script" ] || { echo "GitHub release publisher is missing" >&2; exit 1; }

version=0.1.0-test
artifacts="$tmp/dist"
remote_assets="$tmp/remote"
mkdir "$artifacts" "$remote_assets"
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
  printf '%s\n' "$name" > "$artifacts/$name"
done
(
  cd "$artifacts"
  if command -v sha256sum >/dev/null 2>&1; then sha256sum ./* > checksums.txt; else shasum -a 256 ./* > checksums.txt; fi
)
cp "$artifacts"/* "$remote_assets/"

stable_artifacts="$tmp/stable-dist"
mkdir "$stable_artifacts"
for file in "$artifacts"/*; do
  [ "$(basename -- "$file")" = checksums.txt ] && continue
  name=$(basename -- "$file" | sed 's/0\.1\.0-test/0.1.0/g')
  cp "$file" "$stable_artifacts/$name"
done
(
  cd "$stable_artifacts"
  if command -v sha256sum >/dev/null 2>&1; then sha256sum ./* > checksums.txt; else shasum -a 256 ./* > checksums.txt; fi
)

cat > "$tmp/gh" <<'GH'
#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$AIGW_TEST_GH_LOG"
case "$1 $2" in
  'release view')
    [ "${AIGW_TEST_GH_MODE:-create}" = existing ] && exit 0
    exit 1
    ;;
  'release create')
    shift 2
    if [ "${AIGW_TEST_GH_CREATE_REMOTE:-complete}" = complete ]; then
      for file in "$@"; do
        [ -f "$file" ] || continue
        cp "$file" "$AIGW_TEST_GH_REMOTE/"
      done
    fi
    exit 0
    ;;
  'release download')
    pattern=''
    directory=''
    shift 2
    while [ "$#" -gt 0 ]; do
      case "$1" in
        --pattern) pattern=$2; shift 2 ;;
        --dir) directory=$2; shift 2 ;;
        *) shift ;;
      esac
    done
    mkdir -p "$directory"
    cp "$AIGW_TEST_GH_REMOTE/$pattern" "$directory/$pattern"
    ;;
  *) echo "unexpected gh invocation: $*" >&2; exit 2 ;;
esac
GH
chmod 755 "$tmp/gh"

run() {
  mode=$1
  log=$2
  remote=${3:-$remote_assets}
  create_remote=${4:-complete}
  PATH="$tmp:$PATH" AIGW_TEST_GH_MODE="$mode" AIGW_TEST_GH_LOG="$log" AIGW_TEST_GH_REMOTE="$remote" AIGW_TEST_GH_CREATE_REMOTE="$create_remote" \
    GITHUB_REPOSITORY=owner/aigw-cli CI_COMMIT_TAG="v$version" GH_TOKEN=redacted sh "$script" "$artifacts"
}

created_remote="$tmp/created-remote"
mkdir "$created_remote"
run create "$tmp/create.log" "$created_remote" >/dev/null
grep -q "release create v$version" "$tmp/create.log"
grep -q -- "--verify-tag" "$tmp/create.log"
grep -q -- "--prerelease" "$tmp/create.log"
grep -q "release download v$version --repo owner/aigw-cli --pattern checksums.txt" "$tmp/create.log"
grep -q "release download v$version --repo owner/aigw-cli --pattern aigw_${version}_windows_arm64.zip" "$tmp/create.log"
if grep -q -- '--clobber' "$tmp/create.log"; then
  echo "GitHub publisher may not overwrite an existing release asset" >&2
  exit 1
fi

stable_log="$tmp/stable.log"
stable_remote="$tmp/stable-remote"
mkdir "$stable_remote"
PATH="$tmp:$PATH" AIGW_TEST_GH_MODE=create AIGW_TEST_GH_LOG="$stable_log" AIGW_TEST_GH_REMOTE="$stable_remote" AIGW_TEST_GH_CREATE_REMOTE=complete \
  GITHUB_REPOSITORY=owner/aigw-cli CI_COMMIT_TAG="v0.1.0" GH_TOKEN=redacted sh "$script" "$stable_artifacts" >/dev/null
if grep -q -- '--prerelease' "$stable_log"; then
  echo "GitHub publisher marked a GA release as prerelease" >&2
  exit 1
fi

missing_remote="$tmp/missing-remote"
mkdir "$missing_remote"
if run create "$tmp/missing.log" "$missing_remote" missing >/dev/null 2>&1; then
  echo "GitHub publisher accepted a newly-created release with missing assets" >&2
  exit 1
fi
grep -q "release create v$version" "$tmp/missing.log"
grep -q "release download v$version --repo owner/aigw-cli" "$tmp/missing.log"

run existing "$tmp/existing.log" >/dev/null
grep -q "release download v$version --repo owner/aigw-cli --pattern checksums.txt" "$tmp/existing.log"
grep -q "release download v$version --repo owner/aigw-cli --pattern aigw_${version}_windows_arm64.zip" "$tmp/existing.log"

printf 'tampered\n' > "$remote_assets/aigw_${version}_linux_amd64.tar.gz"
if run existing "$tmp/mismatch.log" >/dev/null 2>&1; then
  echo "GitHub publisher accepted an existing release with mismatched assets" >&2
  exit 1
fi
if grep -q "release create" "$tmp/mismatch.log"; then
  echo "GitHub publisher attempted to replace a mismatched release" >&2
  exit 1
fi

echo "immutable GitHub release publisher contract: OK"
