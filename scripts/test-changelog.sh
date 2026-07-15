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
# The checker must restore the history and release refs from its named origin.
shallow="$tmp/shallow"
origin="$tmp/origin.git"
git init -q --bare "$origin"
git -C "$root" push -q "$origin" 'HEAD:refs/heads/main'
git -C "$root" push -q "$origin" --tags
git -C "$origin" symbolic-ref HEAD refs/heads/main
git clone -q --depth 1 --branch main "file://$origin" "$shallow"
cp "$root/CHANGELOG.md" "$shallow/CHANGELOG.md"
mkdir -p "$shallow/scripts"
cp "$checker" "$shallow/scripts/check-changelog.sh"
(
  cd "$shallow"
  test "$(git rev-parse --is-shallow-repository)" = true
  test -z "$(git tag --list 'v[0-9]*')"
  AIGW_CHANGELOG_FILE=CHANGELOG.md sh scripts/check-changelog.sh
)

# A published entry before the latest tag must be a locally known release rather
# than a speculative version. Appending after the latest heading would be an
# order violation, which is intentionally outside this fixture's scope.
python3 - "$fixture" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
import subprocess

latest_tag = subprocess.check_output(
    ["git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9]*", "HEAD"],
    text=True,
).strip()
latest_version = latest_tag.removeprefix("v")
latest_date = subprocess.check_output(
    ["git", "for-each-ref", f"refs/tags/{latest_tag}", "--format=%(creatordate:short)"],
    text=True,
).strip().splitlines()[0]
marker = f"## [{latest_version}] - {latest_date}"
if marker not in text:
    raise SystemExit("fixture lacks latest release heading")
path.write_text(text.replace(marker, "## [9.9.9] - 2026-07-14\n\n" + marker, 1), encoding="utf-8")
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
changed, count = pattern.subn(r"\1 - 2000-01-01", text, count=1)
if count != 1:
    raise SystemExit("fixture lacks a published release heading")
path.write_text(changed, encoding="utf-8")
PY
if AIGW_CHANGELOG_FILE="$fixture" sh "$checker" >/dev/null 2>&1; then
  echo "changelog checker accepted a tag/date mismatch" >&2
  exit 1
fi

echo "changelog chronology contract: OK"
