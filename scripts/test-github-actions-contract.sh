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
    "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491",
    "go test -race ./...", "go vet ./...", "scripts/check-product-surface.sh", "scripts/check-governance.sh",
    "scripts/check-text-layout.py", "scripts/test-text-layout.sh", "scripts/test-release-source-date-epoch.sh",
    "scripts/test-verified-candidate.sh", "scripts/test-macos-native-install-staging.sh",
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
print("GitHub Actions verification contract: OK")
PYTHON
