#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

require_file() {
  test -f "$1" || { echo "missing governance document: $1" >&2; exit 1; }
}

for file in \
  AGENTS.md \
  CONTRIBUTING.md \
  docs/README.md \
  docs/architecture/authority-and-projection-boundary.md \
  docs/governance/change-and-release-policy.md \
  docs/decisions/0001-control-plane-data-plane-boundary.md \
  docs/evidence/README.md \
  packaging/release/allowed_signers \
  .github/workflows/verify.yml
do
  require_file "$file"
done

sh scripts/check-changelog.sh
sh scripts/check-english-text.sh
python3 scripts/check-text-layout.py

if ! grep -Fq '# AIGW CLI' README.md; then
  echo "README.md must use the formal Project Name as its title" >&2
  exit 1
fi
if ! grep -Fq '`aigw-cli`' README.md; then
  echo "README.md must declare the stable GitLab Path separately" >&2
  exit 1
fi
if ! grep -Fq 'sh scripts/check-governance.sh' .gitlab-ci.yml; then
  echo "GitLab CI must execute the governance check" >&2
  exit 1
fi
if ! grep -Fq 'scripts/check-governance.sh' .github/workflows/verify.yml; then
  echo "GitHub Actions must execute the governance check" >&2
  exit 1
fi
if test -e docs/history || test -e docs/superpowers || test -e docs/design || test -e docs/reviews || test -e docs/specs; then
  echo "retired documentary paths must not remain in the canonical tree" >&2
  exit 1
fi

# AIGW CLI is an English-only repository.  Use explicit Unicode ranges instead
# of a grep Unicode-property dialect so this gate behaves the same on macOS and
# Linux runners.  Test fixtures are included deliberately: they are part of the
# maintainable project surface, not an exemption for stale product copy.
python3 - <<'PY'
from pathlib import Path
import subprocess
import sys

han = set(range(0x3400, 0x4DC0)) | set(range(0x4E00, 0xA000)) | set(range(0xF900, 0xFB00))
matches = []
for name in subprocess.check_output(["git", "ls-files", "-z"]).decode().split("\0"):
    if not name or name == "scripts/check-english-text.sh":
        continue
    path = Path(name)
    try:
        text = path.read_text(encoding="utf-8")
    except (UnicodeDecodeError, OSError):
        continue
    for line_number, line in enumerate(text.splitlines(), 1):
        if any(ord(character) in han for character in line):
            matches.append(f"{name}:{line_number}:{line}")
if matches:
    print("\n".join(matches), file=sys.stderr)
    raise SystemExit("AIGW CLI repository content must be English-only")
PY
