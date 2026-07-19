#!/bin/sh
# Reject credential-shaped values from tracked non-test source files.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

pattern='(sk-[A-Za-z0-9_-]{24,}|Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9_-]{24,})'
if git grep -nE -- "$pattern" -- . ':(exclude)**/*_test.go'; then
  echo "credential-shaped literal found outside test source" >&2
  exit 1
fi

echo "credential literal safety contract: OK"
