#!/bin/sh
# Verify the GitLab module-source fallback semantics without contacting a proxy.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)

python3 - "$root/.gitlab-ci.yml" <<'PYTHON'
from pathlib import Path
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
expected = "${AIGW_GOPROXY:-https://goproxy.cn|https://proxy.golang.org|direct}"
if expected not in text:
    raise SystemExit("GitLab GOPROXY default must use a pipe-separated resilient fallback chain")
if "${AIGW_GOPROXY:-https://goproxy.cn,direct}" in text:
    raise SystemExit("GitLab GOPROXY must not leave a comma-separated timeout-terminal fallback")
print("GitLab Go proxy fallback policy: OK")
PYTHON
