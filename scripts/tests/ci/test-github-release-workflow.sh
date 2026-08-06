#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
workflow="$root/.github/workflows/release.yml"

[ -f "$workflow" ] || { echo "GitHub Actions release workflow is missing" >&2; exit 1; }
python3 - "$workflow" "$root/go.mod" "$root/.config/ci/verify-gates.toml" <<'PYTHON'
from pathlib import Path
import re
import sys
import tomllib
text = Path(sys.argv[1]).read_text(encoding="utf-8")
go_version = next(line.split()[1] for line in Path(sys.argv[2]).read_text(encoding="utf-8").splitlines() if line.startswith("go "))
actions = tomllib.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))["toolchain"]["github_actions"]
checkout_action = actions["checkout"]
setup_go_action = actions["setup_go"]
for key, expected_name in (("checkout", "actions/checkout"), ("setup_go", "actions/setup-go")):
    value = actions[key]
    if re.fullmatch(rf"{re.escape(expected_name)}@[0-9a-f]{{40}}", value) is None:
        raise SystemExit(f"GitHub Action {key} must pin {expected_name} by a 40-character commit SHA")
required = [
    "name: Release", 'tags: ["v*"]', "runs-on: ${{ fromJSON(vars.AIGW_RELEASE_RUNNER) }}",
    "permissions:\n  contents: write",
    checkout_action,
    setup_go_action,
    'scripts/checks/forge/check-release-tag-signature.sh . "$SELECTED_TAG" github', 'AIGW_CHANGELOG_RELEASE_TAG: ${{ inputs.tag || github.ref_name }}', 'git rev-parse "$SELECTED_TAG^{}"', "scripts/checks/release/check-release-toolchain.sh", "go run ./tools/architecture --root .", "scripts/checks/quality/check-static-analysis.sh", "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/release/build/package.sh",
    'SOURCE_DATE_EPOCH="$(sh scripts/release/lib/release-source-date-epoch.sh "$version")"',
    'sh scripts/tests/release/test-release-reproducibility.sh "$version"',
    "scripts/tests/release/test-release-package-layout.sh", "scripts/tests/install/test-macos-native-install-staging.sh", "shell: pwsh", "scripts/tests/install/test-installers.ps1", "publish-github-release.sh",
    "go-version-file: go.mod", "check-latest: false", "cache: false",
    "scripts/tests/release/test-publish-gitlab-release.sh", "scripts/tests/release/test-publish-github-release.sh", "scripts/tests/ci/test-ci-go-cache-preparation.sh",
    "name: Materialize provenance trust input", "AIGW_RELEASE_ALLOWED_SIGNERS: ${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}",
    'allowed_signers="$RUNNER_TEMP/aigw-allowed-signers"', 'printf \'%s\\n\' "$AIGW_RELEASE_ALLOWED_SIGNERS" > "$allowed_signers"',
    'echo "AIGW_GITHUB_ALLOWED_SIGNERS=$allowed_signers" >> "$GITHUB_ENV"',
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions release contract is missing {token!r}")
if "runs-on: [self-hosted" in text or "aigw-github-release-macos-arm64" in text:
    raise SystemExit("GitHub Actions release hardcodes adopter runner inventory")
if "AIGW_GITHUB_ALLOWED_SIGNERS: ${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}" in text:
    raise SystemExit("GitHub release must not pass trust content where a checker requires a path")
if text.count("name: Materialize provenance trust input") != 2:
    raise SystemExit("GitHub release must materialize trust input once per provenance job")
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
if f'go-version: "{go_version}"' in text or f"GOTOOLCHAIN: go{go_version}" in text:
    raise SystemExit("GitHub Actions release duplicates the project Go version")
if "check-latest: true" in text:
    raise SystemExit("GitHub Actions release must not request a newer Go toolchain")
setup = text.index(setup_go_action)
cache = text.index("cache: false")
build = text.index("name: Install release build tools")
if not setup < cache < build:
    raise SystemExit("GitHub Actions release contract must disable setup-go cache before release packaging")
for forbidden in ("@main", "@master"):
    if forbidden in text.lower():
        raise SystemExit(f"GitHub Actions release contract contains stale {forbidden!r} language")
print("GitHub Actions release contract: OK")
PYTHON
