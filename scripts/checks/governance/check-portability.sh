#!/bin/sh
set -eu

tool_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
target_root=${1:-$tool_root}
cd "$target_root"

go -C "$tool_root" run ./tools/architecture \
  --root "$target_root" \
  --policy "$tool_root/.config/checks/architecture/policy.toml"

if grep -RInE \
  'AIGW_(GITLAB|GITHUB)_(AUTHOR_(NAME|EMAIL)|SIGNING_KEY):-[^}]' \
  scripts/forge scripts/release scripts/checks/forge 2>/dev/null; then
  echo "portability contract failed: publication actor identity must be explicit execution input" >&2
  exit 1
fi

if git ls-files '.config/release/*allowed-signers' | grep -q .; then
  echo "portability contract failed: publication trust anchors must be protected execution inputs" >&2
  exit 1
fi

if git grep -nIE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|ssh-(ed25519|rsa)' -- \
  ':!**/*_test.go' ':!scripts/tests/**' ':!CHANGELOG.md' ':!docs/evidence/**' \
  ':!LICENSE' >/dev/null; then
  git grep -nIE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|ssh-(ed25519|rsa)' -- \
    ':!**/*_test.go' ':!scripts/tests/**' ':!CHANGELOG.md' ':!docs/evidence/**' \
    ':!LICENSE' >&2
  echo "portability contract failed: personal identity or key material leaked outside isolated tests" >&2
  exit 1
fi

if grep -RInE \
  '(aigw-(release|github-(verify|release))-macos-arm64|runs-on:[[:space:]]*\[self-hosted)' \
  .config/ci .github .gitlab-ci.yml 2>/dev/null; then
  echo "portability contract failed: CI runner inventory must be supplied by the adopting Forge" >&2
  exit 1
fi

echo "portability contract: OK"
