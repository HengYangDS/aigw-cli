#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-credential-literals.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-credential-literals.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

(
  cd "$root"
  sh "$checker"
)

copy="$tmp/repository"
git clone -q --no-local "file://$root" "$copy"
cp "$checker" "$copy/scripts/check-credential-literals.sh"
(
  cd "$copy"
  sh scripts/check-credential-literals.sh
)

prefix='sk-'
suffix='abcdefghijklmnopqrstuvwxyz012345'
printf 'package literals

const token = "%s%s"
' "$prefix" "$suffix" > "$copy/internal/credential_literal.go"
git -C "$copy" add internal/credential_literal.go
if (
  cd "$copy"
  sh scripts/check-credential-literals.sh
) >/dev/null 2>&1; then
  echo "credential literal checker accepted a tracked non-test API-key-shaped value" >&2
  exit 1
fi

header='authorization: bearer '
printf 'package literals

const header = "%s%s"
' "$header" "$suffix" > "$copy/internal/credential_literal.go"
git -C "$copy" add internal/credential_literal.go
if (
  cd "$copy"
  sh scripts/check-credential-literals.sh
) >/dev/null 2>&1; then
  echo "credential literal checker accepted a mixed-case HTTP Bearer credential" >&2
  exit 1
fi

echo "credential literal safety regression: OK"
