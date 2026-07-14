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

first_heading=$(awk '/^## / { print; exit }' CHANGELOG.md)
test "$first_heading" = "## Unreleased" || {
  echo "CHANGELOG.md must start its release sections with ## Unreleased" >&2
  exit 1
}

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
if test -e docs/superpowers; then
  echo "docs/superpowers is retired; archive provenance under docs/history instead" >&2
  exit 1
fi

versions=$(awk '
  /^## [0-9]+\.[0-9]+\.[0-9]+/ {
    print $2
  }
' CHANGELOG.md)
previous=""
for version in $versions; do
  old_ifs=$IFS
  IFS=.
  set -- $version
  IFS=$old_ifs
  test "$#" -eq 3 || { echo "invalid release version: $version" >&2; exit 1; }
  key=$(printf '%05d%05d%05d' "$1" "$2" "$3")
  if test -n "$previous" && test "$key" \> "$previous"; then
    echo "published CHANGELOG versions must be descending" >&2
    exit 1
  fi
  previous=$key
done
