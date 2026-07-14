#!/bin/sh
set -eu

# The product has one canonical Account -> Profile -> Route -> Adapter model.
# Retired migration and recovery paths must not return through a refactor.
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

fail() {
  echo "retired-residue check failed: $1" >&2
  exit 1
}

matches=$(git grep -n -E \
  'MigrateLegacyV2|legacyProfile|legacyTOMLConfig|legacyJSONConfig|promoteLegacyProfiles|LegacyBinDir|removeLegacyOwnedClaudeShim|recoverStrippedCodexProjection|removeUnmarkedAIGWProviderTable|LegacyConfigVersion|CurrentConfigVersion|NeedsUpgrade' \
  -- ':!**/*_test.go' ':!scripts/check-retired-residue.sh' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired runtime compatibility code found"; }

matches=$(git grep -n -E 'config migrate|config upgrade|docs/migration\.md|docs/history|dmx-credential|dmx-responses-proxy|127\.0\.0\.1:(8791|8888)' \
  -- README.md docs scripts internal examples CHANGELOG.md ':!scripts/check-retired-residue.sh' ':!scripts/check-governance.sh' ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired command or documentation reference found"; }

matches=$(git grep -n -E '\[accounts\.dmx\]|DMXAPI' \
  -- README.md docs examples cmd internal \
  ':!internal/providers/**' ':!internal/cli/doctor.go' ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "provider identity leaked into product defaults"; }

matches=$(git grep -n -E 'gpt-5\.6-[A-Za-z0-9._-]+-cdx' \
  -- . ':!scripts/check-retired-residue.sh' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired GPT -cdx alias found"; }

matches=$(git grep -n -E 'catalog\.Team|internal/catalog' -- cmd internal README.md docs examples ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "bundled provider catalog reference found"; }

for path in docs/migration.md docs/history internal/manifest/legacy_test.go internal/catalog docs/superpowers/plans/2026-07-11-account-runtime-profile-model.md; do
  [ ! -e "$path" ] || fail "retired file remains: $path"
done

echo "retired compatibility residue: OK"
