#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$root/.github/workflows/verify.yml"

[ -f "$workflow" ] || {
  echo "GitHub Actions verification workflow is missing" >&2
  exit 1
}

python3 - "$workflow" <<'PYTHON'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required = [
    "name: Verify",
    "pull_request:",
    "push:",
    "workflow_dispatch:",
    "permissions:\n  contents: read",
    "actions/checkout@v7",
    "actions/setup-go@v6",
    "go test -race ./...",
    "go vet ./...",
    "scripts/check-governance.sh",
    "scripts/check-markdown-presentation.py",
    "scripts/test-pipeline-gates.sh",
    "scripts/test-publish-github-release.sh",
    "scripts/check-package-runner.sh",
    "scripts/package.sh",
    "scripts/test-release-package-layout.sh",
    "scripts/publish-github-release.sh",
    "AIGW_RELEASE_PROVIDER: github",
    "AIGW_RELEASE_MIRROR_PROVIDER: gitlab",
    "AIGW_GITLAB_RELEASE_HOST",
    "AIGW_GITLAB_RELEASE_PROJECT",
    "actions/upload-artifact@v4",
    "actions/download-artifact@v5",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions contract is missing {token!r}")
if "AIGW_GOPROXY" in text or "goproxy.cn" in text:
    raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if "pull-requests: write" in text:
    raise SystemExit("verification workflow must not grant pull-request write permission")
if text.count("contents: write") != 1:
    raise SystemExit("GitHub release workflow must grant contents: write only to its release job")
if "github.event_name == 'push' && startsWith(github.ref, 'refs/tags/v')" not in text:
    raise SystemExit("GitHub package/release jobs must be tag-only")
print("GitHub Actions verification contract: OK")
PYTHON
