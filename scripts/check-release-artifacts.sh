#!/bin/sh
set -eu

out=${1:?usage: check-release-artifacts.sh <dist-dir> <version>}
version=${2:?usage: check-release-artifacts.sh <dist-dir> <version>}

fail() {
  echo "release artifact matrix failed: $1" >&2
  exit 1
}

[ -d "$out" ] || fail "directory does not exist: $out"

required="
aigw_${version}_darwin_amd64.tar.gz
aigw_${version}_darwin_arm64.tar.gz
aigw_${version}_darwin_universal.pkg
aigw_${version}_linux_amd64.deb
aigw_${version}_linux_amd64.rpm
aigw_${version}_linux_amd64.tar.gz
aigw_${version}_linux_arm64.deb
aigw_${version}_linux_arm64.rpm
aigw_${version}_linux_arm64.tar.gz
aigw_${version}_windows_amd64.msi
aigw_${version}_windows_amd64.zip
aigw_${version}_windows_arm64.msi
aigw_${version}_windows_arm64.zip
aigw_${version}.spdx.json
checksums.txt
"

for name in $required; do
  [ -s "$out/$name" ] || fail "missing or empty artifact: $name"
done

for name in $required; do
  [ "$name" = "checksums.txt" ] && continue
  grep -F "  $name" "$out/checksums.txt" >/dev/null || fail "checksums.txt does not cover $name"
done

count=$(find "$out" -maxdepth 1 -type f | wc -l | tr -d ' ')
[ "$count" -eq 15 ] || fail "expected exactly 15 artifacts, found $count"

echo "release artifact matrix: OK (15 artifacts)"
