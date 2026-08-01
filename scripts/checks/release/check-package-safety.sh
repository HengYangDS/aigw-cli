#!/bin/sh
set -eu

# Native package lifecycle hooks run with installer privileges. They must never
# infer a human user's home directory or mutate a user-owned Claude launcher.
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

if grep -RInE 'for[[:space:]]+home[[:space:]]+in|/Users/\*|/home/\*|\$HOME' \
  packaging scripts/release/build/package.sh; then
  echo "native package lifecycle scripts must not traverse or mutate user homes" >&2
  exit 1
fi

if grep -RInE 'rm[[:space:]].*(claude|AIGW managed Claude launcher)' packaging scripts/release/build/package.sh; then
  echo "native package lifecycle scripts must not remove user Claude launchers" >&2
  exit 1
fi

echo "package lifecycle ownership boundary: OK"
