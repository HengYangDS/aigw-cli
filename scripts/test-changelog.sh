#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-changelog.sh"
fixture=$(mktemp "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
trap 'rm -f "$fixture"' EXIT HUP INT TERM

cp "$root/CHANGELOG.md" "$fixture"
AIGW_CHANGELOG_FILE="$fixture" sh "$checker"

# A published entry must be a tagged release rather than a speculative version.
printf '\n## [9.9.9] - 2026-07-14\n' >> "$fixture"
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
