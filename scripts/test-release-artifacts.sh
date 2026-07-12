#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=0.1.0-test
out=$(mktemp -d)
result=$(mktemp)
trap 'rm -rf "$out" "$result"' EXIT HUP INT TERM

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
"

for name in $required; do
  printf 'fixture:%s\n' "$name" > "$out/$name"
done
for name in $required; do
  printf '%064d  %s\n' 0 "$name"
done > "$out/checksums.txt"

if sh "$root/scripts/check-release-artifacts.sh" "$out" "$version" >"$result" 2>&1; then
  cat "$result" >&2
  echo "release artifact check accepted mismatched checksums" >&2
  exit 1
fi
grep -F "checksum mismatch" "$result" >/dev/null || {
  cat "$result" >&2
  echo "release artifact check did not explain the checksum mismatch" >&2
  exit 1
}

checksum_for() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

: > "$out/checksums.txt"
for name in $required; do
  [ "$name" = "aigw_${version}_darwin_amd64.tar.gz" ] && continue
  printf '%s  %s\n' "$(checksum_for "$out/$name")" "$name" >> "$out/checksums.txt"
done

if sh "$root/scripts/check-release-artifacts.sh" "$out" "$version" >"$result" 2>&1; then
  cat "$result" >&2
  echo "release artifact check accepted checksums that omit an artifact" >&2
  exit 1
fi
grep -F "does not cover aigw_0.1.0-test_darwin_amd64.tar.gz" "$result" >/dev/null || {
  cat "$result" >&2
  echo "release artifact check did not explain the missing checksum entry" >&2
  exit 1
}

echo "release artifact checksum validation: OK"
