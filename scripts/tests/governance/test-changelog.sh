#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
checker="$root/scripts/checks/governance/check-changelog.sh"
workspace=$(mktemp -d "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
trap 'rm -rf "$workspace"' EXIT HUP INT TERM

check() {
  file=$1
  AIGW_CHANGELOG_FILE="$file" AIGW_CHANGELOG_RELEASE_TAG= \
    CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= sh "$checker"
}

cp "$root/CHANGELOG.md" "$workspace/valid.md"
check "$workspace/valid.md"

cat > "$workspace/minimal.md" <<'EOF'
## [Unreleased]

## [1.1.0] - 2026-08-06

## [1.0.0] - 2026-08-05
EOF
check "$workspace/minimal.md"

cat > "$workspace/out-of-order.md" <<'EOF'
## [Unreleased]

## [1.0.0] - 2026-08-05

## [1.1.0] - 2026-08-06
EOF
if check "$workspace/out-of-order.md" >/dev/null 2>&1; then
  echo "changelog checker accepted releases outside semantic-version order" >&2
  exit 1
fi

cat > "$workspace/duplicate.md" <<'EOF'
## [Unreleased]

## [1.0.0] - 2026-08-06

## [1.0.0] - 2026-08-05
EOF
if check "$workspace/duplicate.md" >/dev/null 2>&1; then
  echo "changelog checker accepted duplicate releases" >&2
  exit 1
fi

cat > "$workspace/malformed.md" <<'EOF'
## [Unreleased]

## [next] - tomorrow
EOF
if check "$workspace/malformed.md" >/dev/null 2>&1; then
  echo "changelog checker accepted a malformed release heading" >&2
  exit 1
fi

repository="$workspace/repository"
git -c init.templateDir= init -q "$repository"
git -C "$repository" config user.name test
git -C "$repository" config user.email test@example.invalid
mkdir -p "$repository/scripts/checks/governance"
cp "$checker" "$repository/scripts/checks/governance/check-changelog.sh"
cp "$workspace/minimal.md" "$repository/CHANGELOG.md"
git -C "$repository" add .
git -C "$repository" commit -q -m release
git -C "$repository" tag -a v1.1.0 -m release
(
  cd "$repository"
  AIGW_CHANGELOG_RELEASE_TAG=v1.1.0 sh scripts/checks/governance/check-changelog.sh
)
git -C "$repository" commit --allow-empty -q -m newer
if (
  cd "$repository"
  AIGW_CHANGELOG_RELEASE_TAG=v1.1.0 sh scripts/checks/governance/check-changelog.sh >/dev/null 2>&1
); then
  echo "changelog checker accepted a selected tag that does not identify HEAD" >&2
  exit 1
fi

echo "changelog contract: OK"
