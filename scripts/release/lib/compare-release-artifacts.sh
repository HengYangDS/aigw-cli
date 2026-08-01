#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
left=${1:?usage: compare-release-artifacts.sh <left-dir> <right-dir> <version>}
right=${2:?usage: compare-release-artifacts.sh <left-dir> <right-dir> <version>}
version=${3:?usage: compare-release-artifacts.sh <left-dir> <right-dir> <version>}

validate() {
  directory=$1
  "$root/scripts/checks/release/check-release-artifacts.sh" "$directory" "$version" >/dev/null
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$directory" && sha256sum -c checksums.txt >/dev/null)
  else
    (cd "$directory" && shasum -a 256 -c checksums.txt >/dev/null)
  fi
}

validate "$left"
validate "$right"

names=$(cd "$left" && find . -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)
other_names=$(cd "$right" && find . -maxdepth 1 -type f -exec basename {} \; | LC_ALL=C sort)
[ "$names" = "$other_names" ] || {
  echo "release artifact name sets differ across forge stages" >&2
  exit 1
}

for name in $names; do
  cmp -s "$left/$name" "$right/$name" || {
    echo "release artifact differs across forge stages: $name" >&2
    exit 1
  }
done

echo "cross-forge release artifacts: OK"
