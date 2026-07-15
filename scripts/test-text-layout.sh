#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

cp "$root/scripts/check-text-layout.py" "$tmp/check.py"
mkdir -p "$tmp/repository"
cd "$tmp/repository"
git init -q
git config user.name Test
git config user.email test@example.invalid
mkdir -p docs
printf '# Valid\n\nOne paragraph.\n' > README.md
git add README.md
git commit -qm initial
# The checker discovers its repository root from its own path; mirror the
# production location for a true black-box contract check.
mkdir -p scripts
mv "$tmp/check.py" scripts/check-text-layout.py
python3 scripts/check-text-layout.py >/dev/null
printf '# Invalid\n\n\nDouble gap.\n' > README.md
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted consecutive blank lines' >&2
  exit 1
fi
printf '# Valid\n\nOne paragraph.\n' > README.md
printf 'def one():\n    return 1\n\n\ndef two():\n    return 2\n' > docs/style.py
git add docs/style.py
python3 scripts/check-text-layout.py >/dev/null
printf 'def one():\n    return 1\n\n\n\ndef two():\n    return 2\n' > docs/style.py
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted a third Python separator line' >&2
  exit 1
fi
printf 'def one():\n    return 1\n\ndef two():\n    return 2\n' > docs/style.py
printf '# Invalid\nText with trailing space \n' > README.md
if python3 scripts/check-text-layout.py >/dev/null 2>&1; then
  echo 'text layout checker accepted trailing whitespace' >&2
  exit 1
fi
printf '# Valid\n\n```text\n\n\nLiteral gap retained.\n```\n' > README.md
python3 scripts/check-text-layout.py >/dev/null

echo 'Text layout contract test: OK'
