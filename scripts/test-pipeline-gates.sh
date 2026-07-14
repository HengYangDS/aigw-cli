#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 - "$root/.gitlab-ci.yml" <<'PY'
from pathlib import Path
import sys

lines = Path(sys.argv[1]).read_text().splitlines()

workflow_start = next((i for i, line in enumerate(lines) if line == "workflow:"), None)
if workflow_start is None:
    raise SystemExit("CI must define a workflow gate for pre-tag release branches")
workflow_end = next((i for i in range(workflow_start + 1, len(lines)) if lines[i] and not lines[i].startswith((" ", "\t"))), len(lines))
workflow = "\n".join(lines[workflow_start:workflow_end])
if "CI_COMMIT_BRANCH =~ /^release\\/" not in workflow or "when: never" not in workflow:
    raise SystemExit("CI workflow must suppress untagged release/* branch pipelines; the signed tag is the release verification entry")
if "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\\/" not in workflow:
    raise SystemExit("CI workflow must suppress merge-request pipelines from release/* branches before the signed tag exists")

def section(name):
    start = next(i for i, line in enumerate(lines) if line == f"{name}:")
    end = next((i for i in range(start + 1, len(lines)) if lines[i] and not lines[i].startswith((" ", "\t"))), len(lines))
    return lines[start:end]

default = section("default")
if not any("AIGW_GOPROXY" in line and "goproxy.cn,direct" in line for line in default):
    raise SystemExit("default CI environment must configure an overrideable reachable Go module proxy")

if any(line.strip() == "cache:" for line in default):
    raise SystemExit("default CI must not archive Go caches inside the checkout")
if not any("prepare-ci-go-cache.sh" in line for line in default):
    raise SystemExit("default CI must initialize Go caches through the bounded runner-cache helper")

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
if not any("git fetch --tags --force --prune --prune-tags origin" in line for line in verify):
    raise SystemExit("verify must prune stale runner tags before release chronology validation")
if not any("check-release-tag-signature.sh" in line and "CI_COMMIT_TAG" in line for line in verify):
    raise SystemExit("tag pipelines must verify the exact SSH-signed annotated tag before packaging")
if not any("test-release-tag-signature.sh" in line for line in verify):
    raise SystemExit("verify must exercise unsigned-tag rejection and signed-tag acceptance")
if not any("test-linux-native-install-staging.sh" in line for line in verify):
    raise SystemExit("verify must exercise Linux native-install shared-staging behavior without a Docker daemon")
if not any("check-english-text.sh" in line for line in verify):
    raise SystemExit("verify must reject non-English tracked product text")

package = section("package")
if "    - job: windows-installer-runtime" not in package:
    raise SystemExit("package must explicitly need Windows installer runtime verification")
if "    - job: windows-native-acceptance" not in package or "      optional: true" not in package:
    raise SystemExit("package must gate on native Windows acceptance whenever a Windows runner admits that job")
package_script = "\n".join(package)
if '${CI_COMMIT_TAG:-}' not in package_script:
    raise SystemExit("package must tolerate an unset CI_COMMIT_TAG in non-tag pipelines")
if '0.1.0-${CI_COMMIT_SHORT_SHA}' not in package_script:
    raise SystemExit("package must retain a semver-shaped non-tag build fallback")
if package_script.count('AIGW_RELEASE_HOST="$CI_SERVER_URL"') != 1 or package_script.count('AIGW_RELEASE_PROJECT="$CI_PROJECT_PATH"') != 1:
    raise SystemExit("package must inject the current CI release source instead of a repository-specific host or project")
if 'AIGW_RELEASE_MIRROR_HOST' not in package_script or 'AIGW_RELEASE_MIRROR_PROJECT' not in package_script:
    raise SystemExit("package must embed the optional GitHub mirror identity with the GitLab primary source")

publish = section("publish")
if "    - job: package" not in publish:
    raise SystemExit("publish must remain gated by package")

release = section("release")
if any(line.strip().startswith("image:") for line in release):
    raise SystemExit("shell-runner release job must not rely on an ignored container image")
if not any("publish-release.sh" in line for line in release):
    raise SystemExit("release must call the idempotent GitLab Releases API publisher")
if any("release-cli" in line for line in release):
    raise SystemExit("release job must not depend on unavailable release-cli")

mirror = section("mirror-github")
if "  stage: release" not in mirror or "    - job: package" not in mirror:
    raise SystemExit("GitHub mirror publication must consume the exact verified package artifacts")
if not any('AIGW_GITHUB_MIRROR_ENABLED == "true"' in line for line in mirror):
    raise SystemExit("GitHub mirror publication must remain explicit and opt-in")
if not any("publish-github-release.sh" in line for line in mirror):
    raise SystemExit("GitHub mirror job must call the idempotent mirror publisher")
mirror_publisher = Path(sys.argv[1]).parent / "scripts" / "publish-github-release.sh"
mirror_text = mirror_publisher.read_text()
if "Authorization: Bearer $GITHUB_TOKEN" not in mirror_text or "/releases" not in mirror_text:
    raise SystemExit("GitHub mirror publisher must authenticate and use the Releases API")
if '"draft": True' not in mirror_text or "checksums.txt" not in mirror_text:
    raise SystemExit("GitHub mirror publisher must create checksum-verified draft releases")

publisher = Path(sys.argv[1]).parent / "scripts" / "publish-release.sh"
publisher_text = publisher.read_text()
if "JOB-TOKEN: $CI_JOB_TOKEN" not in publisher_text:
    raise SystemExit("release publisher must authenticate with the CI job token")
if "--request POST" not in publisher_text or "--request PUT" not in publisher_text:
    raise SystemExit("release publisher must create or update the GitLab release")
if "/releases" not in publisher_text or "CI_API_V4_URL" not in publisher_text:
    raise SystemExit("release publisher must call the GitLab Releases API directly")

github = Path(sys.argv[1]).parent / ".github" / "workflows" / "verify.yml"
if not github.exists():
    raise SystemExit("GitHub Actions verification workflow is missing")
github_text = github.read_text()
for token in ["actions/checkout@v7", "actions/setup-go@v6", "go test -race ./...", "scripts/check-governance.sh", "scripts/test-git-provider-identities.sh"]:
    if token not in github_text:
        raise SystemExit(f"GitHub Actions verification workflow must contain {token}")
if "contents: write" in github_text or "pull-requests: write" in github_text:
    raise SystemExit("GitHub Actions verification workflow must be read-only")

print("release pipeline gate contract: OK")
PY
