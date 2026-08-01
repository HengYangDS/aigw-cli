#!/bin/sh
set -eu

# The product has one canonical Account -> Profile -> Route -> Adapter model.
# Retired migration and recovery paths must not return through a refactor.
root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

fail() {
  echo "retired-residue check failed: $1" >&2
  exit 1
}

matches=$(git grep -n -E \
  'MigrateLegacyV2|legacyProfile|legacyTOMLConfig|legacyJSONConfig|promoteLegacyProfiles|LegacyBinDir|removeLegacyOwnedClaudeShim|recoverStrippedCodexProjection|removeUnmarkedAIGWProviderTable|LegacyConfigVersion|CurrentConfigVersion|NeedsUpgrade' \
  -- ':!**/*_test.go' ':!scripts/checks/release/check-retired-residue.sh' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired runtime compatibility code found"; }

matches=$(git grep -n -E 'config migrate|config upgrade|docs/migration\.md|docs/history|dmx-credential|dmx-responses-proxy|127\.0\.0\.1:(8791|8888)' \
  -- README.md docs scripts internal examples CHANGELOG.md ':!scripts/checks/release/check-retired-residue.sh' ':!scripts/checks/governance/check-governance.sh' ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired command or documentation reference found"; }

matches=$(git grep -n -E 'git-github-mirror-sync|github-mirror|forge-peer-sync|agent-forge-peers|no_direct_push_allowed|AIGW_RELEASE_(HOST|PROJECT|PRIMARY|MIRROR)|BuildRelease(Host|Project|Primary|Mirror)|AIGW_(GITLAB|GITHUB)_RELEASE_(HOST|PROJECT)|Build(GitLab|GitHub)Release(Host|Project)' \
  -- README.md docs scripts internal cmd packaging .gitlab-ci.yml .github ':!scripts/checks/release/check-retired-residue.sh' ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "retired one-way forge topology found"; }

matches=$(git grep -n -E '\[accounts\.dmx\]|DMXAPI' \
  -- README.md docs examples cmd internal \
  ':!internal/providers/**' ':!internal/cli/doctor.go' ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "provider identity leaked into product defaults"; }

matches=$(git grep -n -E 'catalog\.Team|internal/catalog' -- cmd internal README.md docs examples ':!**/*_test.go' || true)
[ -z "$matches" ] || { printf '%s\n' "$matches" >&2; fail "bundled provider catalog reference found"; }

for path in docs/migration.md docs/history internal/manifest/legacy_test.go internal/catalog docs/superpowers/plans/2026-07-11-account-runtime-profile-model.md scripts/git-github-mirror-sync.sh scripts/test-github-projection-sync.sh scripts/test-git-provider-identities.sh; do
  [ ! -e "$path" ] || fail "retired file remains: $path"
done
for path in scripts/forge/lib/project-github-forge.sh scripts/tests/forge/test-github-provider-projection.sh; do
  [ -x "$path" ] || fail "required provider projection control is missing: $path"
done

echo "retired compatibility residue: OK"
