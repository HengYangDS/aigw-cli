#!/bin/sh
# Test literals must exercise redaction without resembling live credentials.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

if git grep -nE -- "sk-[A-Za-z0-9_-]{24,}|Authorization:[[:space:]]*Bearer[[:space:]]+[A-Za-z0-9_-]{24,}" -- '*_test.go' ':!vendor/**'; then
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
