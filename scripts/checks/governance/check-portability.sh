#!/bin/sh
set -eu

root=${1:-$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)}
cd "$root"

fail() {
  echo "portability contract failed: $1" >&2
  exit 1
}

inventory=$(mktemp "${TMPDIR:-/tmp}/aigw-portability.XXXXXX")
trap 'rm -f "$inventory"' EXIT HUP INT TERM

# Audit the complete physical candidate, not only the current index.  This
# closes the admission gap where a newly created production file could evade
# portability checks until somebody happened to stage it.
python3 - "$inventory" <<'PY'
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

target = Path(sys.argv[1])
tracked = subprocess.run(
    ["git", "ls-files", "-z"], check=True, stdout=subprocess.PIPE
).stdout.split(b"\0")
untracked = subprocess.run(
    ["git", "ls-files", "--others", "--exclude-standard", "-z"],
    check=True,
    stdout=subprocess.PIPE,
).stdout.split(b"\0")
paths = sorted({raw.decode("utf-8", "surrogateescape") for raw in tracked + untracked if raw})
findings: list[str] = []
for name in paths:
    path = Path(name)
    if path.is_symlink():
        findings.append(f"candidate_symlink:{name}")
if findings:
    print("\n".join(findings), file=sys.stderr)
    raise SystemExit(1)
present = [name for name in paths if Path(name).is_file()]
target.write_bytes(b"\0".join(os.fsencode(name) for name in present) + b"\0")
PY

# Test fixtures exercise path parsing with fictitious users and hosts. Product,
# deployment, publication, tooling, and documentation surfaces must remain
# independent of the machine and person that produced a release.
python3 - "$inventory" <<'PY'
from __future__ import annotations

import re
import sys
from pathlib import Path

paths = Path(sys.argv[1]).read_bytes().split(b"\0")

def is_fixture(name: str) -> bool:
    path = Path(name)
    return (
        name.startswith("scripts/tests/")
        or path.name.endswith("_test.go")
        or "testdata" in path.parts
        or "fixtures" in path.parts
    )

patterns = {
    "absolute user-home path": re.compile(r"(?:/Users/|/home/)[A-Za-z0-9_.-]+/"),
    "absolute Windows user-home path": re.compile(
        r"(?i)(?:[A-Z]:[\\/]+Users[\\/]+)[A-Za-z0-9_.-]+[\\/]"
    ),
    "private IPv4 address": re.compile(
        r"(?<![0-9])(?:10\.(?:\d{1,3}\.){2}\d{1,3}|"
        r"192\.168\.(?:\d{1,3}\.)\d{1,3}|"
        r"172\.(?:1[6-9]|2\d|3[01])\.(?:\d{1,3}\.)\d{1,3})(?![0-9])"
    ),
    "personal SSH path": re.compile(r"(?:^|[\s'\"])(?:~|\$HOME)/\.ssh/[A-Za-z0-9_.-]+"),
}
findings: list[str] = []
for raw in paths:
    if not raw:
        continue
    name = raw.decode()
    if is_fixture(name):
        continue
    path = Path(name)
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        continue
    for line_number, line in enumerate(text.splitlines(), 1):
        for label, pattern in patterns.items():
            if pattern.search(line):
                findings.append(f"{name}:{line_number}: {label}: {line}")

if findings:
    print("\n".join(findings), file=sys.stderr)
    raise SystemExit(1)
PY

# These are current publication-policy inputs, not product defaults. They may
# be supplied by an invoking Forge pipeline, a protected environment, or a
# fixture, but they must not be silently selected by reusable product scripts.
if grep -RInE \
  'AIGW_(GITLAB|GITHUB)_(AUTHOR_(NAME|EMAIL)|SIGNING_KEY):-[^}]' \
  scripts/forge scripts/release scripts/checks/forge 2>/dev/null; then
  fail "publication actor identity must be explicit execution input"
fi

if git ls-files '.config/release/*allowed-signers' | grep -q .; then
  fail "publication trust anchors must be protected execution inputs, not product source"
fi

if git grep -nIE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|ssh-(ed25519|rsa)' -- \
  ':!**/*_test.go' ':!scripts/tests/**' ':!CHANGELOG.md' ':!docs/evidence/**' \
  ':!LICENSE' >/dev/null; then
  git grep -nIE '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|ssh-(ed25519|rsa)' -- \
    ':!**/*_test.go' ':!scripts/tests/**' ':!CHANGELOG.md' ':!docs/evidence/**' \
    ':!LICENSE' >&2
  fail "personal identity or key material leaked outside isolated tests"
fi

if grep -RInE \
  '(aigw-(release|github-(verify|release))-macos-arm64|runs-on:[[:space:]]*\[self-hosted)' \
  .config/ci .github .gitlab-ci.yml 2>/dev/null; then
  fail "CI runner inventory must be supplied by the adopting Forge, not product source"
fi

printf '%s\n' 'portability contract: OK'
