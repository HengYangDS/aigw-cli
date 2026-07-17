#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-changelog.sh"
fixture=$(mktemp "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
tmp=$(mktemp -d "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
trap 'rm -f "$fixture"; rm -rf "$tmp"' EXIT HUP INT TERM

cp "$root/CHANGELOG.md" "$fixture"
AIGW_CHANGELOG_FILE="$fixture" sh "$checker"

# A shallow checkout can omit both the release tag and its historical commit.
# Build a complete, minimal release history first, then clone it shallowly. The
# source checkout may itself be shallow in CI, so it cannot safely serve as the
# fixture's remote history.
shallow="$tmp/shallow"
origin="$tmp/origin.git"
source="$tmp/source"
git init -q -b main "$source"
git -C "$source" config user.name 'AIGW Changelog Test'
git -C "$source" config user.email 'aigw-changelog-test@example.invalid'
printf 'release\n' > "$source/release.txt"
git -C "$source" add release.txt
git -C "$source" commit -qm 'release'
git -C "$source" tag -a v0.1.0-rc.1 -m 'release'
release_date=$(git -C "$source" show -s --format=%cs v0.1.0-rc.1^{})
test -n "$release_date" || { echo "fixture release tag has no date" >&2; exit 1; }
printf 'current\n' >> "$source/release.txt"
git -C "$source" commit -qam 'current'
git init -q --bare "$origin"
git -C "$source" push -q "$origin" 'HEAD:refs/heads/main' --tags
git -C "$origin" symbolic-ref HEAD refs/heads/main
git clone -q --depth 1 --branch main "file://$origin" "$shallow"
cat > "$shallow/CHANGELOG.md" <<EOF
# Changelog

## [Unreleased]

## [0.1.0-rc.1] - $release_date
EOF
mkdir -p "$shallow/scripts"
cp "$checker" "$shallow/scripts/check-changelog.sh"
(
  cd "$shallow"
  test "$(git rev-parse --is-shallow-repository)" = true
  test -z "$(git tag --list 'v[0-9]*')"
  # This fixture intentionally has no selected release tag. Clear the outer
  # CI tag variables so a tag pipeline cannot leak its real release identity
  # into the shallow branch-history assertion.
  CI_COMMIT_TAG= GITHUB_REF_TYPE= GITHUB_REF_NAME= \
    AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh
  if AIGW_CHANGELOG_RELEASE_TAG=v0.1.0-rc.1 AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh >/dev/null 2>&1; then
    echo "changelog checker accepted a selected tag that does not identify shallow HEAD" >&2
    exit 1
  fi
  if AIGW_CHANGELOG_RELEASE_TAG=v0.1.0-rc.2 AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh >/dev/null 2>&1; then
    echo "changelog checker accepted a missing selected release tag" >&2
    exit 1
  fi
)

# A published entry before the latest tag must be a locally known release rather
# than a speculative version. Appending after the latest heading would be an
# order violation, which is intentionally outside this fixture's scope.
python3 - "$fixture" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
import subprocess

latest_tag = subprocess.check_output(
    ["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", "HEAD"],
    text=True,
).strip()
latest_version = latest_tag.removeprefix("v")
marker = re.search(rf"^## \[{re.escape(latest_version)}\] - \d{{4}}-\d{{2}}-\d{{2}}$", text, re.MULTILINE)
if marker is None:
    raise SystemExit("fixture lacks latest release heading")
path.write_text(text[:marker.start()] + "## [9.9.9] - 2026-07-14\n\n" + text[marker.start():], encoding="utf-8")
PY
if AIGW_CHANGELOG_FILE="$fixture" sh "$checker" >/dev/null 2>&1; then
  echo "changelog checker accepted an untagged published version" >&2
  exit 1
fi

cp "$root/CHANGELOG.md" "$fixture"
python3 - "$fixture" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
pattern = re.compile(r"^(## \[(?!Unreleased\])[^]]+\]) - \d{4}-\d{2}-\d{2}$", re.MULTILINE)
changed, count = pattern.subn(r"\1 - 2000-02-30", text, count=1)
if count != 1:
    raise SystemExit("fixture lacks a published release heading")
path.write_text(changed, encoding="utf-8")
PY
if AIGW_CHANGELOG_FILE="$fixture" sh "$checker" >/dev/null 2>&1; then
  echo "changelog checker accepted an invalid release date" >&2
  exit 1
fi

echo "changelog chronology contract: OK"
