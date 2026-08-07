#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

require_file() {
  test -f "$1" || { echo "missing governance document: $1" >&2; exit 1; }
}

for file in \
  AGENTS.md \
  LICENSE \
  CONTRIBUTING.md \
  docs/README.md \
  docs/architecture/authority-and-projection-boundary.md \
  docs/governance/change-and-release-policy.md \
  docs/governance/terminal-experience-contract.md \
  docs/decisions/README.md \
  docs/evidence/README.md \
  docs/concepts/README.md \
  docs/guides/team-rollout.md \
  docs/governance/security.md \
  docs/operations/release-readiness.md \
  .config/checks/architecture/policy.toml \
  .config/checks/coverage/policy.toml \
  .config/ci/verify-gates.toml \
  .github/workflows/verify.yml \
  .githooks/pre-commit \
  .githooks/pre-push \
  .githooks/reference-transaction \
  scripts/checks/forge/check-branch-closeout.sh \
  scripts/checks/forge/check-forge-sync.sh \
  scripts/checks/quality/check-static-analysis.sh \
  scripts/checks/governance/check-module-identity.sh \
  scripts/tests/governance/test-module-identity.sh \
  scripts/checks/governance/check-portability.sh \
  scripts/tests/governance/test-portability.sh \
  scripts/tests/governance/test-git-hooks.sh \
  scripts/checks/forge/check-commit-provenance.sh \
  scripts/checks/forge/check-tag-namespace.sh \
  scripts/tests/forge/test-forge-sync.sh \
  scripts/tests/forge/test-commit-provenance.sh \
  tools/historyreplay/main.go
do
  require_file "$file"
done

for gate in \
  'go run ./tools/architecture --root .' \
  'sh scripts/checks/governance/check-module-identity.sh' \
  'go run ./tools/coveragegate --race' \
  'go vet ./...' \
  'sh scripts/checks/quality/check-static-analysis.sh' \
  'sh scripts/checks/governance/check-portability.sh' \
  'sh scripts/tests/governance/test-portability.sh' \
  'test -z "$(gofmt -l cmd internal tools)"' \
  'sh scripts/checks/governance/check-governance.sh' \
  "AIGW_GITLAB_AUTHOR_EMAIL='<release actor email>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-commit-provenance.sh . gitlab" \
  'sh scripts/tests/forge/test-commit-provenance.sh' \
  'go test ./tools/historyreplay' \
  "AIGW_TAG_NAMESPACE_FORGE='<local|gitlab|github>' AIGW_GITLAB_ALLOWED_SIGNERS='<path>' AIGW_GITHUB_ALLOWED_SIGNERS='<path>' sh scripts/checks/forge/check-tag-namespace.sh" \
  'sh scripts/tests/governance/test-changelog.sh'
do
  for document in CONTRIBUTING.md AGENTS.md README.md; do
    if ! grep -Fxq "$gate" "$document"; then
      echo "$document must list required local verification gate exactly: $gate" >&2
      exit 1
    fi
  done
done

for document in CONTRIBUTING.md AGENTS.md README.md; do
  if grep -Fq 'go test -race ./...' "$document"; then
    echo "$document bypasses the required coverage gate" >&2
    exit 1
  fi
done

sh scripts/checks/governance/check-module-identity.sh
sh scripts/tests/governance/test-module-identity.sh
sh scripts/checks/governance/check-portability.sh
sh scripts/tests/governance/test-portability.sh
sh scripts/tests/governance/test-git-hooks.sh
sh scripts/checks/governance/check-changelog.sh
sh scripts/tests/forge/test-tag-namespace.sh
sh scripts/checks/governance/check-english-text.sh
sh scripts/checks/governance/check-product-surface.sh

if ! grep -Fq '# AIGW CLI' README.md; then
  echo "README.md must use the formal Project Name as its title" >&2
  exit 1
fi
if ! grep -Fq '`aigw-cli`' README.md; then
  echo "README.md must declare the stable GitLab Path separately" >&2
  exit 1
fi
for contract in \
  '`AIGW_SECRET_BACKEND=keyring`' \
  '`AIGW_SECRET_BACKEND=env`' \
  '`AIGW_TOKEN_<ACCOUNT>`' \
  '`AIGW_ACCESSIBLE=1`'
do
  if ! grep -Fq "$contract" README.md; then
    echo "README.md must explain public environment contract: $contract" >&2
    exit 1
  fi
done
if grep -Fq 'AIGW_SECRET_BACKEND=keychain' README.md; then
  echo 'README.md names unsupported secret backend keychain' >&2
  exit 1
fi
if ! grep -Fq 'sh scripts/checks/governance/check-governance.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the governance check" >&2
  exit 1
fi
if ! grep -Fq 'scripts/checks/quality/check-static-analysis.sh' .github/workflows/verify.yml; then
  echo "GitHub Actions must execute the static-analysis check" >&2
  exit 1
fi
if ! grep -Fq 'scripts/checks/quality/check-static-analysis.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the static-analysis check" >&2
  exit 1
fi
if ! grep -Fq 'go tool staticcheck' scripts/checks/quality/check-static-analysis.sh || ! grep -Fq 'go tool errcheck ./...' scripts/checks/quality/check-static-analysis.sh; then
  echo "static-analysis check must run the tracked Staticcheck and Errcheck tools" >&2
  exit 1
fi
if ! grep -Fq 'scripts/checks/governance/check-governance.sh' .github/workflows/verify.yml; then
  echo "GitHub Actions must execute the governance check" >&2
  exit 1
fi
if test -e docs/history || test -e docs/superpowers || test -e docs/design || test -e docs/reviews || test -e docs/specs; then
  echo "retired documentary paths must not remain in the canonical tree" >&2
  exit 1
fi
if ! grep -Fxq '.serena/' .gitignore; then
  echo ".gitignore must exclude local Serena project metadata" >&2
  exit 1
fi

go run ./tools/repositorycheck --root "$root" english-text
