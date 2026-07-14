#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

matches=$(git grep -n -I -E '[一-龥㐀-䶿豈-﫿]' -- \
  AGENTS.md CONTRIBUTING.md README.md CHANGELOG.md docs examples cmd internal packaging scripts .gitlab-ci.yml \
  ':!scripts/check-english-text.sh' || true)
[ -z "$matches" ] || {
  printf '%s\n' "$matches" >&2
  echo "English text check failed: tracked product text must be English-only" >&2
  exit 1
}

echo "English text contract: OK"
