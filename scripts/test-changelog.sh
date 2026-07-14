#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
checker="$root/scripts/check-changelog.sh"
fixture=$(mktemp "${TMPDIR:-/tmp}/aigw-changelog.XXXXXX")
trap 'rm -f "$fixture"' EXIT HUP INT TERM

cp "$root/CHANGELOG.md" "$fixture"
AIGW_CHANGELOG_FILE="$fixture" sh "$checker"

# A published entry before the latest tag must be a locally known release rather
# than a speculative version. Appending after the latest heading would be an
# order violation, which is intentionally outside this fixture's scope.
python3 - "$fixture" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
marker = "## [0.1.0-rc.44] - 2026-07-14"
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
