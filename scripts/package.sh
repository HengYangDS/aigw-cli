#!/bin/sh
set -eu

version=${1:-0.1.0-dev}
out=${2:-dist}
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

case "$version" in
  *[!0-9A-Za-z._-]*) echo "invalid version: $version" >&2; exit 2 ;;
esac

rm -rf "$out"
mkdir -p "$out"

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  goos=${target%/*}
  goarch=${target#*/}
  name="aigw_${version}_${goos}_${goarch}"
  stage="$out/$name"
  binary=aigw
  [ "$goos" = windows ] && binary=aigw.exe
  mkdir -p "$stage"
  printf 'building %s/%s\n' "$goos" "$goarch"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X gitlab.local/dig/misc/agentic-third-party-api/aigw-cli/internal/cli.Version=$version" \
    -o "$stage/$binary" ./cmd/aigw
  cp "$root/README.md" "$stage/"
  cp "$root/scripts/install.sh" "$root/scripts/uninstall.sh" "$stage/"
  cp "$root/scripts/install.ps1" "$root/scripts/uninstall.ps1" "$stage/"
  if [ "$goos" = windows ]; then
    (cd "$out" && zip -qr "$name.zip" "$name")
  else
    (cd "$out" && tar -czf "$name.tar.gz" "$name")
  fi
  rm -rf "$stage"
done

(cd "$out" && {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz ./*.zip > checksums.txt
  else
    shasum -a 256 ./*.tar.gz ./*.zip > checksums.txt
  fi
})

go run ./tools/sbom -version "$version" > "$out/aigw_${version}.spdx.json"
printf 'release artifacts written to %s\n' "$out"
