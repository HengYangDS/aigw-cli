#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-credential-fixtures.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-credential-fixtures.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

(
  cd "$root"
  sh "$checker"
)

copy="$tmp/repository"
git clone -q --no-local "file://$root" "$copy"
cp "$checker" "$copy/scripts/check-credential-fixtures.sh"
(
  cd "$copy"
  sh scripts/check-credential-fixtures.sh
)

prefix='sk-'
suffix='abcdefghijklmnopqrstuvwxyz012345'
printf 'package fixtures\n\nconst token = "%s%s"\n' "$prefix" "$suffix" > "$copy/internal/credential_shape_test.go"
git -C "$copy" add internal/credential_shape_test.go
if (
  cd "$copy"
  sh scripts/check-credential-fixtures.sh
) >/dev/null 2>&1; then
  echo "credential fixture checker accepted an API-key-shaped test literal" >&2
  exit 1
fi

header='authorization: bearer '
printf 'package fixtures\n\nconst header = "%s%s"\n' "$header" "$suffix" > "$copy/internal/credential_shape_test.go"
git -C "$copy" add internal/credential_shape_test.go
if (
  cd "$copy"
  sh scripts/check-credential-fixtures.sh
) >/dev/null 2>&1; then
  echo "credential fixture checker accepted a mixed-case HTTP Bearer credential" >&2
  exit 1
fi

echo "credential fixture safety regression: OK"
