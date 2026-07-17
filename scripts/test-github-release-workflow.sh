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
    "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491",
    "scripts/check-release-tag-signature.sh", "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh",
    'SOURCE_DATE_EPOCH="$(sh scripts/release-source-date-epoch.sh "$version")"',
    'sh scripts/test-release-reproducibility.sh "$version"',
    "scripts/test-release-package-layout.sh", "scripts/test-macos-native-install-staging.sh", "shell: pwsh", "scripts/test-installers.ps1", "publish-github-release.sh",
    "AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY",
    "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions release contract is missing {token!r}")
required_toolchain = "brew install nfpm msitools"
if required_toolchain not in text:
    raise SystemExit("GitHub Actions release contract must install the Homebrew msitools formula")
if "brew install nfpm wixl" in text or "brew tap msitools/msitools" in text:
    raise SystemExit("GitHub Actions release contract retains an obsolete wixl installation path")
for forbidden in ("@main", "@master"):
    if forbidden in text.lower():
        raise SystemExit(f"GitHub Actions release contract contains stale {forbidden!r} language")
print("GitHub Actions release contract: OK")
PYTHON
