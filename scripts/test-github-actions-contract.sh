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
    "scripts/test-git-provider-identities.sh",
    "scripts/test-pipeline-gates.sh",
    "scripts/test-publish-github-release.sh",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions contract is missing {token!r}")
if "AIGW_GOPROXY" in text or "goproxy.cn" in text:
    raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if "pull-requests: write" in text or "contents: write" in text:
    raise SystemExit("verification workflow must use read-only repository permissions")
print("GitHub Actions verification contract: OK")
PYTHON
