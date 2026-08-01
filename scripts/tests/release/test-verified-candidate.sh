#!/bin/sh
# Contract test for the offline, checksum-first candidate carrier.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
packager="$root/scripts/release/build/package-verified-candidate.sh"
checker="$root/scripts/checks/release/check-verified-candidate.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-candidate-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

[ -x "$packager" ] || { echo "candidate packager is missing" >&2; exit 1; }
[ -x "$checker" ] || { echo "candidate checker is missing" >&2; exit 1; }

version=0.1.0-candidate.1
dist="$tmp/dist"
out="$tmp/out"
mkdir -p "$dist" "$out"

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
  printf 'fixture:%s\n' "$name" > "$dist/$name"
done
(cd "$dist" && {
  files=$(find . -maxdepth 1 -type f ! -name checksums.txt | sort)
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum $files
  else
    shasum -a 256 $files
  fi
} > checksums.txt)
sh "$root/scripts/checks/release/check-release-artifacts.sh" "$dist" "$version" >/dev/null

# Package from a clean temporary clone; the carrier must not alter the source
# formal matrix or place candidate files inside it.
clone="$tmp/clone"
git clone -q --no-local "$root" "$clone"
mkdir -p "$clone/scripts/release/build" "$clone/scripts/checks/release"
cp "$packager" "$clone/scripts/release/build/package-verified-candidate.sh"
cp "$checker" "$root/scripts/checks/release/check-release-artifacts.sh" "$clone/scripts/checks/release/"
chmod 755 \
  "$clone/scripts/release/build/package-verified-candidate.sh" \
  "$clone/scripts/checks/release/check-release-artifacts.sh" \
  "$clone/scripts/checks/release/check-verified-candidate.sh"
(cd "$clone" && sh scripts/release/build/package-verified-candidate.sh "$version" "$dist" "$out") >/dev/null

carrier=$(find "$out" -maxdepth 1 -name '*.tar.gz' -type f -print -quit)
[ -n "$carrier" ] || { echo "candidate carrier was not created" >&2; exit 1; }
[ -s "$carrier.sha256" ] || { echo "candidate outer checksum was not created" >&2; exit 1; }
sh "$clone/scripts/checks/release/check-verified-candidate.sh" "$carrier" >/dev/null
[ "$(find "$dist" -maxdepth 1 -type f | wc -l | tr -d ' ')" = 15 ] || {
  echo "candidate carrier modified formal dist" >&2
  exit 1
}

# Changing the packaged manifest invalidates its declared digest.
stage="$tmp/stage"
mkdir -p "$stage"
tar -xzf "$carrier" -C "$stage"
payload=$(find "$stage" -mindepth 1 -maxdepth 1 -type d -print -quit)
printf 'tampered\n' >> "$payload/artifacts/checksums.txt"
tampered="$tmp/tampered.tar.gz"
(cd "$stage" && tar -czf "$tampered" "$(basename "$payload")")
if sh "$clone/scripts/checks/release/check-verified-candidate.sh" "$tampered" >/dev/null 2>&1; then
  echo "candidate checker accepted a tampered manifest" >&2
  exit 1
fi

# Unsafe archive members must fail before extraction.
unsafe="$tmp/unsafe.tar.gz"
python3 - "$unsafe" <<'PY'
import io
import tarfile
import sys

with tarfile.open(sys.argv[1], "w:gz") as archive:
    info = tarfile.TarInfo("../escape")
    data = b"unsafe\n"
    info.size = len(data)
    archive.addfile(info, io.BytesIO(data))
PY
if sh "$clone/scripts/checks/release/check-verified-candidate.sh" "$unsafe" >/dev/null 2>&1; then
  echo "candidate checker accepted an unsafe archive member" >&2
  exit 1
fi

echo "verified candidate carrier contract: OK"
