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

checksum_entries=$(mktemp)
required_names=$(mktemp)
trap 'rm -f "$checksum_entries" "$required_names"' EXIT HUP INT TERM

for name in $required; do
  [ "$name" = "checksums.txt" ] && continue
  printf '%s\n' "$name" >> "$required_names"
done

awk '
  NF != 2 { bad=1; next }
  $1 !~ /^[0-9A-Fa-f]{64}$/ { bad=1; next }
  {
    name=$2
    sub(/^\.\//, "", name)
    if (name == "" || seen[name]++) { duplicate=name; bad=1; next }
    print tolower($1) " " name
  }
  END { if (bad) exit 1 }
' "$out/checksums.txt" > "$checksum_entries" || {
  duplicate=$(awk 'NF == 2 {name=$2; sub(/^\.\//, "", name); if (seen[name]++) {print name; exit}}' "$out/checksums.txt")
  [ -z "$duplicate" ] || fail "duplicate checksum entry: $duplicate"
  fail "invalid checksum manifest format"
}

for name in $(cat "$required_names"); do
  expected=$(awk -v name="$name" '$2 == name {print $1; exit}' "$checksum_entries")
  [ -n "$expected" ] || fail "checksums.txt does not cover $name"
  actual=$(checksum_for "$out/$name")
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $name"
done

actual_entries=$(wc -l < "$checksum_entries" | tr -d ' ')
expected_entries=$(wc -l < "$required_names" | tr -d ' ')
[ "$actual_entries" -eq "$expected_entries" ] || {
  unexpected=$(awk 'NR == FNR { wanted[$1]=1; next } !wanted[$2] {print $2; exit}' "$required_names" "$checksum_entries")
  fail "unexpected checksum entry: ${unexpected:-unknown}"
}

count=$(find "$out" -maxdepth 1 -type f | wc -l | tr -d ' ')
[ "$count" -eq 15 ] || fail "expected exactly 15 artifacts, found $count"

echo "release artifact matrix: OK (15 artifacts)"
