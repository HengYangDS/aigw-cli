#!/bin/sh
set -eu

document=${1:?usage: check-release-readiness-doc.sh <release-readiness.md>}

if [ ! -r "$document" ]; then
  echo "cannot read release evidence contract: $document" >&2
  exit 2
fi

if grep -n -E 'Current status \(20[0-9][0-9]-[0-9][0-9]-[0-9][0-9]\)|0\.1\.0-rc\.[0-9]+|codex/initial-product|GitLab SSH|GitLab API|e082b00' "$document"; then
  echo "release evidence contract contains a stale release snapshot" >&2
  exit 1
fi
