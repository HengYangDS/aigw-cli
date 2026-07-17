#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-0.1.0-reproducibility-test}
epoch=${SOURCE_DATE_EPOCH:-1784246400}
tmp=$(mktemp -d)
first="$tmp/first"
second="$tmp/second"
success=0
trap 'status=$?; if [ "$success" = 1 ]; then rm -rf "$tmp"; else echo "reproducibility outputs retained: $tmp" >&2; fi; exit "$status"' EXIT HUP INT TERM

case "$epoch" in
  ''|*[!0-9]*) echo "SOURCE_DATE_EPOCH must be a non-negative Unix epoch" >&2; exit 2 ;;
esac

build() {
  out=$1
  env \
    SOURCE_DATE_EPOCH="$epoch" \
    AIGW_GITLAB_RELEASE_ORIGIN=https://gitlab.example.test \
    AIGW_GITLAB_RELEASE_REPOSITORY=example-group/aigw-cli \
    AIGW_GITHUB_RELEASE_ORIGIN=https://github.com \
    AIGW_GITHUB_RELEASE_REPOSITORY=example-owner/aigw-cli \
    AIGW_REQUIRE_FULL_MATRIX=1 \
    sh "$root/scripts/package.sh" "$version" "$out" >/dev/null
  sh "$root/scripts/test-release-package-layout.sh" "$out" "$version" >/dev/null
}

build "$first"
build "$second"

first_names=$(cd "$first" && find . -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)
second_names=$(cd "$second" && find . -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)
[ "$first_names" = "$second_names" ] || {
  echo "release artifact name sets differ" >&2
  exit 1
}

for name in $first_names; do
  cmp -s "$first/$name" "$second/$name" || {
    echo "release artifact is not reproducible: $name" >&2
    exit 1
  }
done

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$first" && sha256sum -c checksums.txt >/dev/null)
  (cd "$second" && sha256sum -c checksums.txt >/dev/null)
else
  (cd "$first" && shasum -a 256 -c checksums.txt >/dev/null)
  (cd "$second" && shasum -a 256 -c checksums.txt >/dev/null)
fi

success=1
echo "release reproducibility contract: OK"
