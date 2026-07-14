#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

require_file() {
  test -f "$1" || { echo "missing governance document: $1" >&2; exit 1; }
}

for file in \
  AGENTS.md \
  CONTRIBUTING.md \
  docs/README.md \
  docs/architecture/authority-and-projection-boundary.md \
  docs/governance/change-and-release-policy.md \
  docs/decisions/0001-control-plane-data-plane-boundary.md \
  docs/evidence/README.md
do
  require_file "$file"
done

sh scripts/check-changelog.sh

if ! grep -Fq '# AIGW CLI' README.md; then
  echo "README.md must use the formal Project Name as its title" >&2
  exit 1
fi
if ! grep -Fq '`aigw-cli`' README.md; then
  echo "README.md must declare the stable GitLab Path separately" >&2
  exit 1
fi
if ! grep -Fq 'sh scripts/check-governance.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the governance check" >&2
  exit 1
fi
if test -e docs/superpowers || test -e docs/design || test -e docs/reviews || test -e docs/specs; then
  echo "retired execution-document paths must be moved under docs/history" >&2
  exit 1
fi
