#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
version=${1:?usage: release-source-date-epoch.sh <version> [changelog]}
changelog=${2:-"$root/CHANGELOG.md"}

[ -f "$changelog" ] || {
  echo "changelog does not exist: $changelog" >&2
  exit 2
}

python3 - "$version" "$changelog" <<'PY'
from __future__ import annotations

import calendar
import datetime as dt
import re
import sys
from pathlib import Path

version, changelog = sys.argv[1:]
pattern = re.compile(r"^## \[" + re.escape(version) + r"\] - (\d{4}-\d{2}-\d{2})$")
matches = [match.group(1) for line in Path(changelog).read_text(encoding="utf-8").splitlines() if (match := pattern.fullmatch(line))]

if not matches:
    print(f"release heading not found: {version}", file=sys.stderr)
    raise SystemExit(2)
if len(matches) != 1:
    print(f"release heading must occur exactly once: {version}", file=sys.stderr)
    raise SystemExit(2)

raw_date = matches[0]
try:
    parsed = dt.date.fromisoformat(raw_date)
except ValueError:
    print(f"invalid release date: {raw_date}", file=sys.stderr)
    raise SystemExit(2)

print(calendar.timegm(dt.datetime.combine(parsed, dt.time.min, tzinfo=dt.timezone.utc).timetuple()))
PY
