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

default = section("default")
if not any("AIGW_GOPROXY" in line and "goproxy.cn,direct" in line for line in default):
    raise SystemExit("default CI environment must configure an overrideable reachable Go module proxy")

if not any("GOFLAGS" in line and "-modcacherw" in line for line in default):
    raise SystemExit("default CI environment must make the workspace module cache removable")

if any(line.strip() == "cache:" for line in default):
    raise SystemExit("default CI must not archive Go caches inside the checkout")
if not any("AIGW_CI_CACHE_ROOT" in line and "CI_BUILDS_DIR" in line for line in default):
    raise SystemExit("default CI environment must keep Go caches in a runner-owned location")
if any('GOMODCACHE="$CI_PROJECT_DIR' in line or 'GOCACHE="$CI_PROJECT_DIR' in line for line in default):
    raise SystemExit("default CI Go caches must not live under CI_PROJECT_DIR")

runtime = section("windows-installer-runtime")
if "  stage: verify" not in runtime:
    raise SystemExit("windows installer runtime verification must remain a verify-stage job")
if "  tags: [macos]" not in runtime:
    raise SystemExit("PowerShell installer contract verification must remain on the macOS release runner")
if not any("command -v pwsh" in line for line in runtime):
    raise SystemExit("windows installer runtime verification must fail closed when pwsh is unavailable")
if not any("test-installers.ps1" in line for line in runtime):
    raise SystemExit("windows installer runtime verification must execute the native PowerShell harness")

native = section("windows-native-acceptance")
if "  stage: verify" not in native:
    raise SystemExit("native Windows acceptance must be a verify-stage job")
if "  tags: [windows]" not in native:
    raise SystemExit("native Windows acceptance must require a Windows-tagged runner")
if not any('AIGW_WINDOWS_NATIVE_RUNNER == "true"' in line for line in native):
    raise SystemExit("native Windows acceptance must remain disabled until a real Windows runner is explicitly admitted")
if not any("test-windows-native.ps1" in line for line in native):
    raise SystemExit("native Windows acceptance must execute the Windows-only acceptance harness")

verify = section("verify")
if not any("test-linux-native-install-staging.sh" in line for line in verify):
    raise SystemExit("verify must exercise Linux native-install shared-staging behavior without a Docker daemon")

package = section("package")
if "    - job: windows-installer-runtime" not in package:
    raise SystemExit("package must explicitly need Windows installer runtime verification")
if "    - job: windows-native-acceptance" not in package or "      optional: true" not in package:
    raise SystemExit("package must gate on native Windows acceptance whenever a Windows runner admits that job")

publish = section("publish")
if "    - job: package" not in publish:
    raise SystemExit("publish must remain gated by package")

release = section("release")
if any(line.strip().startswith("image:") for line in release):
    raise SystemExit("shell-runner release job must not rely on an ignored container image")
if not any("JOB-TOKEN: $CI_JOB_TOKEN" in line for line in release):
    raise SystemExit("release must authenticate with the CI job token")
if not any("/releases" in line and "CI_API_V4_URL" in line for line in release):
    raise SystemExit("release must call the GitLab Releases API directly")
if any("release-cli" in line for line in release):
    raise SystemExit("release job must not depend on unavailable release-cli")

print("release pipeline gate contract: OK")
PY
