#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$root/.github/workflows/release.yml"

[ -f "$workflow" ] || { echo "GitHub Actions release workflow is missing" >&2; exit 1; }
python3 - "$workflow" <<'PYTHON'
from pathlib import Path
import sys
text = Path(sys.argv[1]).read_text(encoding="utf-8")
required = [
    "name: Release", 'tags: ["v*"]', "runs-on: macos-15",
    "permissions:\n  contents: write",
    "actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd",
    "actions/setup-go@d35c59abb061a4a6fb18e82ac0862c26744f8676",
    "scripts/check-release-tag-signature.sh", "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh",
    "scripts/test-release-package-layout.sh", "publish-github-release.sh",
    "AIGW_GITHUB_RELEASE_HOST", "AIGW_GITHUB_RELEASE_PROJECT",
    "AIGW_GITLAB_RELEASE_HOST", "AIGW_GITLAB_RELEASE_PROJECT",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions release contract is missing {token!r}")
for forbidden in ("@main", "@master", "mirror", "primary"):
    if forbidden in text.lower():
        raise SystemExit(f"GitHub Actions release contract contains stale {forbidden!r} language")
print("GitHub Actions release contract: OK")
PYTHON
