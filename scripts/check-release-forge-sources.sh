#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
python3 "$root/scripts/resolve-release-forge-sources.py" \
  --file "$root/packaging/release/forge-sources.env" >/dev/null

echo "release forge-source manifest: OK"
