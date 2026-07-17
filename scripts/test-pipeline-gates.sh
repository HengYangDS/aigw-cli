#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

python3 - "$root/.gitlab-ci.yml" "$root/.github/workflows/release.yml" <<'PYTHON'
from pathlib import Path
import re
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
if "image: golang:1.25.8" not in default:
    raise SystemExit("GitLab release toolchain must pin Go 1.25.8 exactly")
if "AIGW_GOPROXY" not in default or "prepare-ci-go-cache.sh" not in default:
    raise SystemExit("GitLab must retain its independently configured Go dependency path")
if "https://goproxy.cn|https://proxy.golang.org|direct" not in default:
    raise SystemExit("GitLab Go dependency path must fall back after transient proxy failures")
if "tags: [aigw-release-macos-arm64]" not in default:
    raise SystemExit("GitLab must schedule the full pipeline on its dedicated release runner")

variables = section(gitlab, "variables")
if "GOTOOLCHAIN: go1.25.8" not in variables:
    raise SystemExit("GitLab must resolve Go 1.25.8 on every runner")

verify = section(gitlab, "verify")
for required in [
    "go test -race ./...", "go vet ./...", "check-product-surface.sh", 'check-release-tag-signature.sh . "$CI_COMMIT_TAG" gitlab', "check-release-toolchain.sh",
    "check-english-text.sh", "test-linux-native-install-staging.sh", "test-macos-native-install-staging.sh",
    "test-verified-candidate.sh",
    "test-publish-release.sh", "test-publish-github-release.sh",
    "test-pipeline-gates.sh", "test-github-actions-contract.sh",
    "test-github-release-workflow.sh",
    "test-github-provider-projection.sh", "check-text-layout.py", "test-text-layout.sh",
    "test-ci-go-proxy-policy.sh", "test-ci-go-cache-preparation.sh",
    "test-release-source-date-epoch.sh", "test-release-forge-sources.sh", "test-release-toolchain.sh",
]:
    if required not in verify:
        raise SystemExit(f"GitLab verification is missing {required}")

package = section(gitlab, "package")
if "macos-native-acceptance" in package:
    raise SystemExit("package must not depend on post-package macOS native acceptance")
for required in [
    "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh",
    'SOURCE_DATE_EPOCH="$(git log -1 --format=%ct)"',
    'SOURCE_DATE_EPOCH="$(sh scripts/release-source-date-epoch.sh "$VERSION")"',
    'if test -n "${CI_COMMIT_TAG:-}"; then',
    'sh scripts/test-release-reproducibility.sh "$VERSION"',
]:
    if required not in package:
        raise SystemExit(f"GitLab package plane is missing {required}")
for forbidden in [
    'AIGW_GITLAB_RELEASE_ORIGIN="$CI_SERVER_URL"',
    'AIGW_GITLAB_RELEASE_REPOSITORY="$CI_PROJECT_PATH"',
    'AIGW_GITHUB_RELEASE_ORIGIN', 'AIGW_GITHUB_RELEASE_REPOSITORY',
]:
    if forbidden in package:
        raise SystemExit(f"GitLab package plane retains provider-specific build metadata: {forbidden}")

release = section(gitlab, "release")
if "publish-release.sh" not in release or "needs: [publish]" not in release:
    raise SystemExit("GitLab must publish its own independently built release")
if "mirror-github:" in gitlab or "AIGW_GITHUB_MIRROR" in gitlab:
    raise SystemExit("GitLab CI must not retain a one-way GitHub dependency")

windows_native = section(gitlab, "windows-native-acceptance")
if "allow_failure: true" not in windows_native:
    raise SystemExit("Windows native evidence must not block RC publication")
macos_native = section(gitlab, "macos-native-acceptance")
for required in ["stage: acceptance", "allow_failure: true", "artifacts: true", "AIGW_MACOS_ACCEPTANCE_USER", "AIGW_MACOS_UPGRADE_PACKAGE", "test-macos-native-install.sh"]:
    if required not in macos_native:
        raise SystemExit(f"macOS native evidence job is missing {required}")
for section_text, name in [
    (section(gitlab, "windows-installer-runtime"), "Windows installer"),
    (macos_native, "macOS native acceptance"),
    (package, "package"),
]:
    if "tags: [aigw-release-macos-arm64]" not in section_text:
        raise SystemExit(f"{name} must use the dedicated release runner")

for required in [
    "name: Release", 'tags: ["v*"]', "permissions:\n  contents: write",
    "runs-on: [self-hosted, macOS, ARM64, aigw-release-macos-arm64]", 'check-release-tag-signature.sh . "$SELECTED_TAG" github',
    'go-version: "1.25.8"', "check-latest: false", "GOTOOLCHAIN: go1.25.8", "check-release-toolchain.sh",
    "AIGW_REQUIRE_FULL_MATRIX=1 sh scripts/package.sh", "publish-github-release.sh",
    'SOURCE_DATE_EPOCH="$(sh scripts/release-source-date-epoch.sh "$version")"',
    'sh scripts/test-release-reproducibility.sh "$version"',
    "check-text-layout.py", "test-text-layout.sh",
    "test-verified-candidate.sh", "test-macos-native-install-staging.sh",
    "test-publish-release.sh", "test-publish-github-release.sh", "test-ci-go-cache-preparation.sh",
]:
    if required not in github:
        raise SystemExit(f"GitHub independent release plane is missing {required}")
for forbidden in [
    "AIGW_GITLAB_RELEASE_ORIGIN", "AIGW_GITLAB_RELEASE_REPOSITORY",
    "AIGW_GITHUB_RELEASE_ORIGIN", "AIGW_GITHUB_RELEASE_REPOSITORY",
]:
    if forbidden in github:
        raise SystemExit(f"GitHub release plane retains provider-specific build metadata: {forbidden}")
if "gitlab-ci" in github.lower() or re.search(r"(?m)^\s*sh scripts/publish-release\.sh(?:\s|$)", github):
    raise SystemExit("GitHub release plane retains a non-peer dependency")
for workflow_name, workflow_text in (("GitHub release", github),):
    if "go-version-file:" in workflow_text or "check-latest: true" in workflow_text:
        raise SystemExit(f"{workflow_name} retains floating Go setup")

print("dual forge CI/CD contract: OK")
PYTHON
