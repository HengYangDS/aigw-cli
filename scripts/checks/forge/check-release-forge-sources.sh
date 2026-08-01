#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
python3 "$root/scripts/release/lib/resolve-release-forge-sources.py" \
  --file "$root/.config/release/forge-sources.env" >/dev/null

echo "release forge-source schema fixture: OK"
