#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
workflow="$root/.github/workflows/release.yml"

[ -f "$workflow" ] || { echo "GitHub Actions release workflow is missing" >&2; exit 1; }
python3 - "$workflow" "$root/go.mod" <<'PYTHON'
from pathlib import Path
import sys
text = Path(sys.argv[1]).read_text(encoding="utf-8")
go_version = next(line.split()[1] for line in Path(sys.argv[2]).read_text(encoding="utf-8").splitlines() if line.startswith("go "))
required = [
    "name: Release", 'tags: ["v*"]', "runs-on: ${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}",
    "permissions:\n  contents: write",
    "actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd",
    "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491",
    'scripts/checks/forge/check-release-tag-signature.sh . "$SELECTED_TAG" github', 'AIGW_CHANGELOG_RELEASE_TAG: ${{ inputs.tag || github.ref_name }}', 'git rev-parse "$SELECTED_TAG^{}"', "scripts/checks/release/check-release-toolchain.sh", "go run ./tools/architecture --root .", "scripts/checks/quality/check-static-analysis.sh", "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/release/build/package.sh",
    'SOURCE_DATE_EPOCH="$(sh scripts/release/lib/release-source-date-epoch.sh "$version")"',
    'sh scripts/tests/release/test-release-reproducibility.sh "$version"',
    "scripts/tests/release/test-release-package-layout.sh", "scripts/tests/install/test-macos-native-install-staging.sh", "shell: pwsh", "scripts/tests/install/test-installers.ps1", "publish-github-release.sh",
    f'go-version: "{go_version}"', "check-latest: false", "cache: false", f"GOTOOLCHAIN: go{go_version}",
    "scripts/tests/release/test-publish-gitlab-release.sh", "scripts/tests/release/test-publish-github-release.sh", "scripts/tests/ci/test-ci-go-cache-preparation.sh",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions release contract is missing {token!r}")
if "runs-on: [self-hosted" in text or "aigw-github-release-macos-arm64" in text:
    raise SystemExit("GitHub Actions release hardcodes adopter runner inventory")
required_toolchain = "brew install nfpm msitools"
if required_toolchain not in text:
    raise SystemExit("GitHub Actions release contract must install the Homebrew msitools formula")
if "brew install nfpm wixl" in text or "brew tap msitools/msitools" in text:
    raise SystemExit("GitHub Actions release contract retains an obsolete wixl installation path")
for required_input in (
    "AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY",
    "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY",
    "AIGW_PACKAGE_HOMEPAGE",
):
    if required_input not in text:
        raise SystemExit(f"GitHub Actions release contract omits explicit release input: {required_input}")
if "go-version-file:" in text or "check-latest: true" in text:
    raise SystemExit("GitHub Actions release contract retains floating Go configuration")
setup = text.index("actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491")
cache = text.index("cache: false")
build = text.index("name: Install release build tools")
if not setup < cache < build:
    raise SystemExit("GitHub Actions release contract must disable setup-go cache before release packaging")
for forbidden in ("@main", "@master"):
    if forbidden in text.lower():
        raise SystemExit(f"GitHub Actions release contract contains stale {forbidden!r} language")
print("GitHub Actions release contract: OK")
PYTHON
