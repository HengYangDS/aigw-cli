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
    end = next((i for i in range(start + 1, len(lines)) if lines[i].strip() and not lines[i].lstrip().startswith("#") and not lines[i].startswith((" ", "\t"))), len(lines))
    return "\n".join(lines[start:end])

workflow = section(gitlab, "workflow")
if r"CI_COMMIT_BRANCH =~ /^release\/" not in workflow or "when: never" not in workflow:
    raise SystemExit("GitLab must suppress untagged release branch pipelines")
if r"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\/" not in workflow:
    raise SystemExit("GitLab must suppress release branch merge-request pipelines")

default = section(gitlab, "default")
if "image: golang:1.25.12" not in default:
    raise SystemExit("GitLab release toolchain must pin Go 1.25.12 exactly")
if "AIGW_GOPROXY" not in default or "prepare-ci-go-cache.sh" not in default:
    raise SystemExit("GitLab must retain its independently configured Go dependency path")
if "https://goproxy.cn|https://proxy.golang.org|direct" not in default:
    raise SystemExit("GitLab Go dependency path must fall back after transient proxy failures")
if "tags: [aigw-release-macos-arm64]" not in default:
    raise SystemExit("GitLab must schedule the full pipeline on its dedicated release runner")

variables = section(gitlab, "variables")
if 'GIT_DEPTH: "0"' not in variables:
    raise SystemExit("GitLab CI must declare complete history for release chronology")
if "GOTOOLCHAIN: go1.25.12" not in variables:
    raise SystemExit("GitLab must resolve Go 1.25.12 on every runner")

verify = section(gitlab, "verify")
if "git fetch --tags --force origin" not in verify:
    raise SystemExit("GitLab verification must refresh its release tags without pruning GitHub provenance")
if "--prune-tags origin" in verify or "--prune origin" in verify:
    raise SystemExit("GitLab verification must not prune local GitHub provenance namespaces")
for required in [
    "go test -race ./...", "go vet ./...", "check-static-analysis.sh", "check-product-surface.sh", 'check-release-tag-signature.sh . "$CI_COMMIT_TAG" gitlab', "check-release-toolchain.sh",
    "check-english-text.sh", "test-linux-native-install-staging.sh", "test-macos-native-install-staging.sh",
    "test-verified-candidate.sh",
    "test-publish-release.sh", "test-publish-github-release.sh",
    "test-pipeline-gates.sh", "test-github-actions-contract.sh",
    "test-github-release-workflow.sh",
    "test-github-provider-projection.sh", "test-tag-namespace.sh", "check-text-layout.py", "test-text-layout.sh",
    "test-ci-go-proxy-policy.sh", "test-ci-go-cache-preparation.sh",
    "test-release-source-date-epoch.sh", "test-release-forge-sources.sh", "test-release-toolchain.sh", "test-release-tag-signature-provider-selection.sh",
]:
    if required not in verify:
        raise SystemExit(f"GitLab verification is missing {required}")
verify_commands = [
    "sh scripts/check-credential-literals.sh",
    "sh scripts/test-credential-literals.sh",
    "sh scripts/check-credential-fixtures.sh",
    "sh scripts/test-credential-fixtures.sh",
    "sh scripts/test-branch-closeout.sh",
]

def check_canonical_mapping_keys(lines, indent, banned, context):
    for line in lines:
        if len(line) - len(line.lstrip()) != indent:
            continue
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        merge = re.match(r"^<<\s*:", stripped)
        match = re.match(r"^([A-Za-z0-9_-]+)\s*:", stripped)
        if merge is None and match is None:
            raise SystemExit(f"{context} contains non-canonical mapping key syntax: {stripped}")
        key = "<<" if merge is not None else match.group(1)
        if key in banned:
            raise SystemExit(f"{context} must not define {key}")

def check_verify_commands(verify_job):
    lines = verify_job.splitlines()
    job_indent = len(lines[0]) - len(lines[0].lstrip())
    check_canonical_mapping_keys(
        lines[1:],
        job_indent + 2,
        {"allow_failure", "rules", "only", "except", "when", "extends", "<<"},
        "GitLab verify job",
    )
    script_start = next(
        index for index, line in enumerate(lines) if line == "  script:"
    )
    script_indent = len(lines[script_start]) - len(lines[script_start].lstrip())
    script_end = next(
        (
            index
            for index in range(script_start + 1, len(lines))
            if lines[index].strip()
            and not lines[index].lstrip().startswith("#")
            and len(lines[index]) - len(lines[index].lstrip()) <= script_indent
        ),
        len(lines),
    )
    verify_script = "\n".join(lines[script_start + 1:script_end])
    for command in verify_commands:
        if not re.search(rf"(?m)^[ \t]*-[ \t]+{re.escape(command)}[ \t]*$", verify_script):
            raise SystemExit(f"GitLab verification is missing active command: {command}")

check_verify_commands(verify)

for key, body in [
    ("allow_failure", "  allow_failure: true\n"),
    ("rules", "  rules:\n    - when: never\n"),
    ("extends", "  extends: .nonblocking\n"),
]:
    candidate = verify.replace("verify:\n", f"verify:\n{body}", 1)
    try:
        check_verify_commands(candidate)
    except SystemExit as error:
        expected = f"GitLab verify job must not define {key}"
        if str(error) != expected:
            raise SystemExit(f"GitLab {key} fixture failed unexpectedly: {error}")
    else:
        raise SystemExit(f"GitLab contract accepted verify job with {key}")

for body in [
    '  "allow\\u005ffailure": true\n',
    "  'allow_failure' : true\n",
]:
    candidate = verify.replace("verify:\n", f"verify:\n{body}", 1)
    try:
        check_verify_commands(candidate)
    except SystemExit as error:
        expected = f"GitLab verify job contains non-canonical mapping key syntax: {body.strip()}"
        if str(error) != expected:
            raise SystemExit(f"GitLab non-canonical key fixture failed unexpectedly: {error}")
    else:
        raise SystemExit("GitLab contract accepted a non-canonical verify job key")

verify_end = gitlab.index("\nwindows-installer-runtime:")
commented_gitlab = (
    gitlab[:verify_end]
    + "\n# low-indent comment\n  allow_failure: true"
    + gitlab[verify_end:]
)
try:
    check_verify_commands(section(commented_gitlab, "verify"))
except SystemExit as error:
    expected = "GitLab verify job must not define allow_failure"
    if str(error) != expected:
        raise SystemExit(f"GitLab low-indent comment fixture failed unexpectedly: {error}")
else:
    raise SystemExit("GitLab contract accepted allow_failure after a low-indent comment")

# GitLab after_script failures do not gate the job, so command text there must
# not satisfy the verify.script execution contract.
inactive_command = verify_commands[0]
inactive_line = f"    - {inactive_command}\n"
if verify.count(inactive_line) != 1:
    raise SystemExit("GitLab active-command fixture has an unexpected command count")
inactive = verify.replace(inactive_line, "", 1)
inactive += f"\n  after_script:\n    - {inactive_command}"
try:
    check_verify_commands(inactive)
except SystemExit as error:
    expected = f"GitLab verification is missing active command: {inactive_command}"
    if str(error) != expected:
        raise SystemExit(f"GitLab after_script fixture failed for an unexpected reason: {error}")
else:
    raise SystemExit("GitLab contract accepted a verification gate present only in after_script")

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
for required in [
    "publish-release.sh dist",
    "job: publish",
    "job: package",
    "artifacts: true",
]:
    if required not in release:
        raise SystemExit(
            f"GitLab release verification is missing its local artifact matrix contract: {required}"
        )
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
    "runs-on: [self-hosted, macOS, ARM64, aigw-github-release-macos-arm64]", 'check-release-tag-signature.sh . "$SELECTED_TAG" github',
    'go-version: "1.25.12"', "check-latest: false", "cache: false", "GOTOOLCHAIN: go1.25.12", "check-release-toolchain.sh", "check-static-analysis.sh",
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
    "aigw-gitlab-macos-arm64",
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
