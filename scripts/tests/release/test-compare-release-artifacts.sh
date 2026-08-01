#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
version=0.1.0-cross-forge-test
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
left="$tmp/left"
right="$tmp/right"
mkdir "$left" "$right"

names="
aigw_${version}.spdx.json
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
"

for directory in "$left" "$right"; do
  for name in $names; do printf '%s\n' "$name" > "$directory/$name"; done
  (cd "$directory" && shasum -a 256 $names > checksums.txt)
done

"$root/scripts/release/lib/compare-release-artifacts.sh" "$left" "$right" "$version" >/dev/null
printf 'different\n' >> "$right/aigw_${version}_windows_arm64.zip"
(cd "$right" && shasum -a 256 $names > checksums.txt)
if "$root/scripts/release/lib/compare-release-artifacts.sh" "$left" "$right" "$version" >"$tmp/different.out" 2>&1; then
  echo "cross-forge comparison accepted a changed artifact" >&2
  exit 1
fi
grep -Fx "release artifact differs across forge stages: aigw_${version}_windows_arm64.zip" "$tmp/different.out" >/dev/null
echo "cross-forge release comparison contract: OK"
