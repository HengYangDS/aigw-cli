#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

# Product text must remain English-only. Use Unicode code-point ranges rather
# than locale-sensitive grep ranges: CI and contributor machines may expose
# different collation rules, while the policy must have one result everywhere.
python3 - <<'PY'
from pathlib import Path
import subprocess

roots = {
    "AGENTS.md", "CONTRIBUTING.md", "README.md", "CHANGELOG.md", "docs",
    "examples", "cmd", "internal", "packaging", "scripts", ".gitlab-ci.yml",
}
han = set(range(0x3400, 0x4DC0)) | set(range(0x4E00, 0xA000)) | set(range(0xF900, 0xFB00))
matches = []
for name in subprocess.check_output(["git", "ls-files", "-z"]).decode().split("\0"):
    if not name or name == "scripts/checks/governance/check-english-text.sh":
        continue
    if not any(name == root or name.startswith(root + "/") for root in roots):
        continue
    path = Path(name)
    try:
        text = path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        continue
    for number, line in enumerate(text.splitlines(), 1):
        if any(ord(character) in han for character in line):
            matches.append(f"{name}:{number}:{line}")
if matches:
    print("\n".join(matches))
    raise SystemExit("English text check failed: tracked product text must be English-only")
PY

echo "English text contract: OK"
