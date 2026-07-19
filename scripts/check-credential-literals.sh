#!/bin/sh
# Reject credential-shaped values from tracked non-test source files.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

sk_pattern='sk-[A-Za-z0-9_-]{24,}'
bearer_pattern='Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9_-]{24,}'
if git grep -nE -- "$sk_pattern" -- . ':(exclude)**/*_test.go' ||
  git grep -niE -- "$bearer_pattern" -- . ':(exclude)**/*_test.go'; then
  echo "credential-shaped literal found outside test source" >&2
  exit 1
fi

echo "credential literal safety contract: OK"
