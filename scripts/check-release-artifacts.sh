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

checksum_for() {
  file=$1
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

for name in $required; do
  [ "$name" = "checksums.txt" ] && continue
  expected=$(awk -v name="$name" '$2 == name || $2 == "./" name {print $1; exit}' "$out/checksums.txt")
  [ -n "$expected" ] || fail "checksums.txt does not cover $name"
  actual=$(checksum_for "$out/$name")
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $name"
done

count=$(find "$out" -maxdepth 1 -type f | wc -l | tr -d ' ')
[ "$count" -eq 15 ] || fail "expected exactly 15 artifacts, found $count"

echo "release artifact matrix: OK (15 artifacts)"
