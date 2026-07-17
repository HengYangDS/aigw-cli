#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$root/.github/workflows/verify.yml"

[ -f "$workflow" ] || { echo "GitHub Actions verification workflow is missing" >&2; exit 1; }
python3 - "$workflow" <<'PYTHON'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required = [
    "name: Verify", "pull_request:", "push:", "workflow_dispatch:",
    "permissions:\n  contents: read",
    "actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd",
    "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491", 'go-version: "1.25.8"', "check-latest: false", "GOTOOLCHAIN: go1.25.8", "git fetch --force --tags origin", "if: github.ref_type == 'tag'", 'SELECTED_TAG: ${{ github.ref_name }}', 'scripts/check-release-tag-signature.sh . "$SELECTED_TAG" github', "scripts/check-release-toolchain.sh",
    "go test -race ./...", "go vet ./...", "scripts/check-product-surface.sh", "scripts/check-governance.sh",
    "scripts/check-text-layout.py", "scripts/test-text-layout.sh", "scripts/test-release-source-date-epoch.sh",
    "scripts/test-verified-candidate.sh", "scripts/test-release-tag-signature-provider-selection.sh", "scripts/test-macos-native-install-staging.sh",
    "shell: pwsh", "scripts/test-installers.ps1",
    "scripts/test-ci-go-cache-preparation.sh",
    "scripts/test-publish-release.sh", "scripts/test-publish-github-release.sh",
    "scripts/test-pipeline-gates.sh", "scripts/test-github-release-workflow.sh",
    "scripts/test-github-provider-projection.sh",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions contract is missing {token!r}")
if "AIGW_GOPROXY" in text or "goproxy.cn" in text:
    raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if "pull-requests: write" in text or "contents: write" in text:
    raise SystemExit("verification workflow must use read-only repository permissions")
if "@main" in text or "@master" in text:
    raise SystemExit("GitHub Actions must use immutable action revisions")
if "go-version-file:" in text or "check-latest: true" in text:
    raise SystemExit("GitHub Actions verification must not float its Go toolchain")
checkout = text.index("name: Check out full history and tags")
refresh = text.index("git fetch --force --tags origin")
provenance = text.index("name: Verify pushed release tag provenance")
gates = text.index("name: Run source and policy gates")
if not checkout < refresh < provenance < gates:
    raise SystemExit("GitHub Actions must refresh and verify annotated tags before source gates")
print("GitHub Actions verification contract: OK")
PYTHON
