#!/bin/sh
set -eu

# The package job is a controlled macOS shell runner. It must possess every
# build and inspection utility needed to create and validate the full release
# matrix before any artifact work begins.
required_commands="
go
lipo
pkgbuild
productbuild
nfpm
wixl
msibuild
xar
file
tar
zip
unzip
ar
bsdtar
msiextract
msiinfo
pkgutil
"

missing=0
for command in $required_commands; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing package-runner command: $command" >&2
    missing=1
  fi
done

if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo "missing package-runner SHA-256 utility: sha256sum or shasum" >&2
  missing=1
fi

[ "$missing" -eq 0 ] || exit 1
echo "package runner capability: OK"
