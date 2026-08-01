#!/bin/sh
# Test literals must exercise redaction without resembling live credentials.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

sk_pattern='sk-[A-Za-z0-9_-]{24,}'
bearer_pattern='Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9_-]{24,}'
if git grep -nE -- "$sk_pattern" -- '*_test.go' ':!vendor/**' ||
  git grep -niE -- "$bearer_pattern" -- '*_test.go' ':!vendor/**'; then
  echo "credential-shaped test fixture found; use an aigw-test-* sentinel instead" >&2
  exit 1
fi

git grep -Fq 'aigw-test-secret-never-leaks' -- internal/secrets/store_test.go || {
  echo "secret-store redaction test must retain an explicit sentinel" >&2
  exit 1
}
git grep -Fq 'aigw-test-gateway-token-never-leaks' -- internal/diagnostics/probe_test.go || {
  echo "gateway redaction test must retain an explicit sentinel" >&2
  exit 1
}

echo "credential fixture safety contract: OK"
