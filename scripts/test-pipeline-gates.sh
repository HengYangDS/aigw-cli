#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 - "$root/.gitlab-ci.yml" "$root/.github/workflows/release.yml" <<'PYTHON'
from pathlib import Path
import sys

gitlab = Path(sys.argv[1]).read_text(encoding="utf-8")
github = Path(sys.argv[2]).read_text(encoding="utf-8")

def section(text, name):
    lines = text.splitlines()
    start = next(i for i, line in enumerate(lines) if line == f"{name}:")
    end = next((i for i in range(start + 1, len(lines)) if lines[i] and not lines[i].startswith((" ", "\t"))), len(lines))
    return "\n".join(lines[start:end])

workflow = section(gitlab, "workflow")
if r"CI_COMMIT_BRANCH =~ /^release\/" not in workflow or "when: never" not in workflow:
    raise SystemExit("GitLab must suppress untagged release branch pipelines")
if r"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\/" not in workflow:
    raise SystemExit("GitLab must suppress release branch merge-request pipelines")

default = section(gitlab, "default")
if "AIGW_GOPROXY" not in default or "prepare-ci-go-cache.sh" not in default:
    raise SystemExit("GitLab must retain its independently configured Go dependency path")

verify = section(gitlab, "verify")
for required in [
    "go test -race ./...", "go vet ./...", "check-release-tag-signature.sh",
    "check-english-text.sh", "test-linux-native-install-staging.sh",
    "test-pipeline-gates.sh", "test-github-actions-contract.sh",
    "test-github-release-workflow.sh", "test-forge-peer-sync.sh",
    "check-text-layout.py", "test-text-layout.sh",
]:
    if required not in verify:
        raise SystemExit(f"GitLab verification is missing {required}")

package = section(gitlab, "package")
for required in [
    'AIGW_GITLAB_RELEASE_HOST="$CI_SERVER_URL"',
    'AIGW_GITLAB_RELEASE_PROJECT="$CI_PROJECT_PATH"',
    "AIGW_GITHUB_RELEASE_HOST", "AIGW_GITHUB_RELEASE_PROJECT",
    "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh",
]:
    if required not in package:
        raise SystemExit(f"GitLab package plane is missing {required}")

release = section(gitlab, "release")
if "publish-release.sh" not in release or "needs: [publish]" not in release:
    raise SystemExit("GitLab must publish its own independently built release")
if "mirror-github:" in gitlab or "AIGW_GITHUB_MIRROR" in gitlab:
    raise SystemExit("GitLab CI must not retain a one-way GitHub dependency")

for required in [
    "name: Release", 'tags: ["v*"]', "permissions:\n  contents: write",
    "runs-on: macos-15", "check-release-tag-signature.sh",
    "AIGW_GITHUB_RELEASE_HOST", "AIGW_GITHUB_RELEASE_PROJECT",
    "AIGW_GITLAB_RELEASE_HOST", "AIGW_GITLAB_RELEASE_PROJECT",
    "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh", "publish-github-release.sh",
    "check-text-layout.py", "test-text-layout.sh",
]:
    if required not in github:
        raise SystemExit(f"GitHub independent release plane is missing {required}")
if "gitlab-ci" in github.lower() or "publish-release.sh" in github or "mirror" in github.lower() or "primary" in github.lower():
    raise SystemExit("GitHub release plane retains a non-peer dependency")

print("dual forge CI/CD contract: OK")
PYTHON
