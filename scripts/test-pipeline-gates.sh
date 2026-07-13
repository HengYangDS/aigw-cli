#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 - "$root/.gitlab-ci.yml" <<'PY'
from pathlib import Path
import sys

lines = Path(sys.argv[1]).read_text().splitlines()

def section(name):
    start = next(i for i, line in enumerate(lines) if line == f"{name}:")
    end = next((i for i in range(start + 1, len(lines)) if lines[i] and not lines[i].startswith((" ", "\t"))), len(lines))
    return lines[start:end]

runtime = section("windows-installer-runtime")
if "  stage: verify" not in runtime:
    raise SystemExit("windows installer runtime verification must remain a verify-stage job")
if "  tags: [macos]" not in runtime:
    raise SystemExit("windows installer runtime verification must run on the macOS release runner")
if not any("command -v pwsh" in line for line in runtime):
    raise SystemExit("windows installer runtime verification must fail closed when pwsh is unavailable")
if not any("test-installers.ps1" in line for line in runtime):
    raise SystemExit("windows installer runtime verification must execute the native PowerShell harness")

package = section("package")
if "    - job: windows-installer-runtime" not in package:
    raise SystemExit("package must explicitly need Windows installer runtime verification")

publish = section("publish")
if "    - job: package" not in publish:
    raise SystemExit("publish must remain gated by package")

print("release pipeline gate contract: OK")
PY
