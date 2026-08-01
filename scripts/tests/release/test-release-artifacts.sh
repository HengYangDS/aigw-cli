#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
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

checksum_for() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

write_valid_manifest() {
  : > "$out/checksums.txt"
  for name in $required; do
    printf '%s  %s\n' "$(checksum_for "$out/$name")" "$name" >> "$out/checksums.txt"
  done
}

expect_reject() {
  reason=$1
  if sh "$root/scripts/checks/release/check-release-artifacts.sh" "$out" "$version" >"$result" 2>&1; then
    cat "$result" >&2
    echo "release artifact check accepted $reason" >&2
    exit 1
  fi
}

expect_message() {
  message=$1
  grep -F "$message" "$result" >/dev/null || {
    cat "$result" >&2
    echo "release artifact check did not explain: $message" >&2
    exit 1
  }
}

for name in $required; do
  printf 'fixture:%s\n' "$name" > "$out/$name"
done

# An incorrect digest must never validate merely because its filename exists.
for name in $required; do
  printf '%064d  %s\n' 0 "$name"
done > "$out/checksums.txt"
expect_reject "mismatched checksums"
expect_message "checksum mismatch"

# A missing required record must identify the omitted artifact.
write_valid_manifest
sed '/aigw_0.1.0-test_darwin_amd64.tar.gz/d' "$out/checksums.txt" > "$out/checksums.tmp"
mv "$out/checksums.tmp" "$out/checksums.txt"
expect_reject "checksums that omit an artifact"
expect_message "does not cover aigw_0.1.0-test_darwin_amd64.tar.gz"

# A manifest is an exact index: duplicate and foreign records are ambiguous.
write_valid_manifest
first=$(awk 'NR == 1 {print; exit}' "$out/checksums.txt")
printf '%s\n' "$first" >> "$out/checksums.txt"
expect_reject "a duplicate checksum entry"
expect_message "duplicate checksum entry"

write_valid_manifest
printf '%064d  unexpected-artifact\n' 0 >> "$out/checksums.txt"
expect_reject "an unexpected checksum entry"
expect_message "unexpected checksum entry"

# The accepted spelling may include an optional ./ prefix, as produced by
# common checksum tools when invoked from a release directory.
write_valid_manifest
first_name=$(printf '%s\n' "$required" | awk 'NF {print; exit}')
sed "s#  $first_name#  ./$first_name#" "$out/checksums.txt" > "$out/checksums.tmp"
mv "$out/checksums.tmp" "$out/checksums.txt"
sh "$root/scripts/checks/release/check-release-artifacts.sh" "$out" "$version" >/dev/null

echo "release artifact checksum validation: OK"
