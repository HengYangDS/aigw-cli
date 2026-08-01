#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
config="$root/.config/ci/verify-gates.toml"
gitlab_file="$root/.gitlab-ci.yml"
github_verify_file="$root/.github/workflows/verify.yml"
github_release_file="$root/.github/workflows/release.yml"
go_mod="$root/go.mod"

for required in "$config" "$gitlab_file" "$github_verify_file" "$github_release_file" "$go_mod"; do
  test -f "$required" || {
    echo "pipeline gates SSOT input missing: $required" >&2
    exit 1
  }
done

python3 - "$root" "$config" "$gitlab_file" "$github_verify_file" "$github_release_file" "$go_mod" <<'PYTHON'
from __future__ import annotations

from pathlib import Path
import re
import sys
import tomllib

root = Path(sys.argv[1])
config_path = Path(sys.argv[2])
gitlab_path = Path(sys.argv[3])
github_verify_path = Path(sys.argv[4])
github_release_path = Path(sys.argv[5])
go_mod_path = Path(sys.argv[6])

with config_path.open("rb") as handle:
    gates = tomllib.load(handle)

gitlab = gitlab_path.read_text(encoding="utf-8")
github_verify = github_verify_path.read_text(encoding="utf-8")
github_release = github_release_path.read_text(encoding="utf-8")

def go_version_from_mod(path: Path) -> str:
    for line in path.read_text(encoding="utf-8").splitlines():
        parts = line.split()
        if len(parts) >= 2 and parts[0] == "go":
            return parts[1]
    raise SystemExit(f"go.mod has no Go version: {path}")

def section(text: str, name: str) -> str:
    lines = text.splitlines()
    start = next(i for i, line in enumerate(lines) if line == f"{name}:")
    end = next(
        (
            i
            for i in range(start + 1, len(lines))
            if lines[i].strip()
            and not lines[i].lstrip().startswith("#")
            and not lines[i].startswith((" ", "\t"))
        ),
        len(lines),
    )
    return "\n".join(lines[start:end])

def require_tokens(text: str, tokens: list[str], context: str) -> None:
    for token in tokens:
        if token not in text:
            raise SystemExit(f"{context} is missing {token!r}")

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

def check_verify_commands(verify_job: str, required_commands: list[str]) -> None:
    lines = verify_job.splitlines()
    job_indent = len(lines[0]) - len(lines[0].lstrip())
    check_canonical_mapping_keys(
        lines[1:],
        job_indent + 2,
        {"allow_failure", "rules", "only", "except", "when", "extends", "<<"},
        "GitLab verify job",
    )
    script_start = next(index for index, line in enumerate(lines) if line == "  script:")
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
    verify_script = "\n".join(lines[script_start + 1 : script_end])
    for command in required_commands:
        if not re.search(rf"(?m)^[ \t]*-[ \t]+{re.escape(command)}[ \t]*$", verify_script):
            raise SystemExit(f"GitLab verification is missing active command: {command}")

go_version = go_version_from_mod(go_mod_path)
toolchain = gates["toolchain"]
common_commands = list(gates["common"]["commands"])
active_commands = list(gates["common"]["active_script_commands"]["commands"])
gitlab_cfg = gates["gitlab"]
gitlab_commands = list(gates["gitlab"]["commands"]["required"])
gitlab_package_required = list(gates["gitlab"]["package"]["required"])
gitlab_release_required = list(gates["gitlab"]["release"]["required"])
github_verify_cfg = gates["github"]["verify"]
github_verify_commands = list(gates["github"]["verify"]["commands"]["required"])
github_release_cfg = gates["github"]["release"]
github_release_commands = list(gates["github"]["release"]["commands"]["required"])
github_release_forbid = list(gates["github"]["release"]["forbid"]["tokens"])
native_linux = gates["native"]["linux"]
native_windows = gates["native"]["windows"]
native_macos = gates["native"]["macos"]

expected_image = f"{toolchain['gitlab_image_prefix']}{go_version}"
expected_gotoolchain = f"{toolchain['gotoolchain_prefix']}{go_version}"
expected_github_go = f'{toolchain["github_go_version_field"]}: "{go_version}"'

# --- GitLab structural contract driven by SSOT ---
workflow = section(gitlab, "workflow")
if gitlab_cfg.get("suppress_untagged_release_branch", False):
    if r"CI_COMMIT_BRANCH =~ /^release\/" not in workflow or "when: never" not in workflow:
        raise SystemExit("GitLab must suppress untagged release branch pipelines")
if gitlab_cfg.get("suppress_release_branch_merge_request", False):
    if r"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME =~ /^release\/" not in workflow:
        raise SystemExit("GitLab must suppress release branch merge-request pipelines")

default = section(gitlab, "default")
if f"image: {expected_image}" not in default:
    raise SystemExit(f"GitLab release toolchain must pin Go {go_version} exactly from go.mod")
if "AIGW_GOPROXY" not in default or gitlab_cfg["prepare_cache_script"] not in default:
    raise SystemExit("GitLab must retain its independently configured Go dependency path")
if gitlab_cfg["goproxy_fallback"] not in default:
    raise SystemExit("GitLab Go dependency path must fall back after transient proxy failures")
expected_gitlab_tag = f"tags: [${gitlab_cfg['runner_tag_variable']}]"
if expected_gitlab_tag not in default:
    raise SystemExit("GitLab must receive its runner tag from protected Forge context")

variables = section(gitlab, "variables")
if f'GIT_DEPTH: "{gitlab_cfg["git_depth"]}"' not in variables:
    raise SystemExit("GitLab CI must declare complete history for release chronology")
if f"GOTOOLCHAIN: {expected_gotoolchain}" not in variables:
    raise SystemExit(f"GitLab must resolve Go {go_version} on every runner")

verify = section(gitlab, "verify")
if "--prune-tags" in verify:
    raise SystemExit("GitLab verification must not prune local GitHub provenance namespaces")

for required in common_commands + gitlab_commands:
    if required not in verify and required not in gitlab:
        # Common and GitLab-specific gates must appear in the verify job body.
        if required not in verify:
            raise SystemExit(f"GitLab verification is missing {required}")

check_verify_commands(verify, active_commands)

for key, body in [
    ("allow_failure", "  allow_failure: true\n"),
    ("rules", "  rules:\n    - when: never\n"),
    ("extends", "  extends: .nonblocking\n"),
]:
    candidate = verify.replace("verify:\n", f"verify:\n{body}", 1)
    try:
        check_verify_commands(candidate, active_commands)
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
        check_verify_commands(candidate, active_commands)
    except SystemExit as error:
        expected = f"GitLab verify job contains non-canonical mapping key syntax: {body.strip()}"
        if str(error) != expected:
            raise SystemExit(f"GitLab non-canonical key fixture failed unexpectedly: {error}")
    else:
        raise SystemExit("GitLab contract accepted a non-canonical verify job key")

verify_end = gitlab.index("\nwindows-installer-runtime:")
commented_gitlab = (
    gitlab[:verify_end] + "\n# low-indent comment\n  allow_failure: true" + gitlab[verify_end:]
)
try:
    check_verify_commands(section(commented_gitlab, "verify"), active_commands)
except SystemExit as error:
    expected = "GitLab verify job must not define allow_failure"
    if str(error) != expected:
        raise SystemExit(f"GitLab low-indent comment fixture failed unexpectedly: {error}")
else:
    raise SystemExit("GitLab contract accepted allow_failure after a low-indent comment")

# GitLab after_script failures do not gate the job, so command text there must
# not satisfy the verify.script execution contract.
inactive_command = active_commands[0]
inactive_line = f"    - {inactive_command}\n"
if verify.count(inactive_line) != 1:
    raise SystemExit("GitLab active-command fixture has an unexpected command count")
inactive = verify.replace(inactive_line, "", 1)
inactive += f"\n  after_script:\n    - {inactive_command}"
try:
    check_verify_commands(inactive, active_commands)
except SystemExit as error:
    expected = f"GitLab verification is missing active command: {inactive_command}"
    if str(error) != expected:
        raise SystemExit(f"GitLab after_script fixture failed for an unexpected reason: {error}")
else:
    raise SystemExit("GitLab contract accepted a verification gate present only in after_script")

package = section(gitlab, "package")
if "macos-native-acceptance" in package:
    raise SystemExit("package must not depend on post-package macOS native acceptance")
require_tokens(package, gitlab_package_required, "GitLab package plane")
for forbidden in gitlab_cfg.get("forbid_package_provider_build_metadata", []):
    if forbidden in package:
        raise SystemExit(f"GitLab package plane retains provider-specific build metadata: {forbidden}")

if gitlab_cfg.get("forbid_windows_native_acceptance_job", False) and "windows-native-acceptance:" in gitlab:
    raise SystemExit("GitLab must not schedule its unmanageable Windows runner")
if gitlab_cfg.get("forbid_macos_native_acceptance_job", False) and "macos-native-acceptance:" in gitlab:
    raise SystemExit("GitLab must not schedule macOS package acceptance without administrator credentials")

release = section(gitlab, "release")
require_tokens(release, gitlab_release_required, "GitLab release verification")
if gitlab_cfg.get("forbid_github_mirror_job", False):
    if "mirror-github:" in gitlab or "AIGW_GITHUB_MIRROR" in gitlab:
        raise SystemExit("GitLab CI must not retain a one-way GitHub dependency")

for section_text, name in [
    (section(gitlab, "windows-installer-runtime"), "Windows installer"),
    (package, "package"),
]:
    if expected_gitlab_tag not in section_text:
        raise SystemExit(f"{name} must use the protected release runner selection")

# --- GitHub verify projection ---
require_tokens(github_verify, common_commands, "GitHub verify plane")
require_tokens(github_verify, github_verify_commands, "GitHub verify plane")
if expected_github_go not in github_verify:
    raise SystemExit(f"GitHub verify must pin go-version {go_version} from go.mod")
if f"GOTOOLCHAIN: {expected_gotoolchain}" not in github_verify:
    raise SystemExit(f"GitHub verify must set GOTOOLCHAIN to {expected_gotoolchain}")
if f"runs-on: {github_verify_cfg['runner']}" not in github_verify:
    raise SystemExit("GitHub verify must receive its runner inventory from repository variables")
if f"permissions:\n  {github_verify_cfg['permissions']}" not in github_verify and (
    f"permissions:\n  {github_verify_cfg['permissions'].replace(': ', ': ')}" not in github_verify
):
    # permissions block is two lines in the workflow
    if "permissions:\n  contents: read" not in github_verify:
        raise SystemExit("GitHub verify must use read-only repository permissions")

if github_verify_cfg.get("forbid_pull_request_triggers", False):
    if "pull_request:" in github_verify or "pull_request_target:" in github_verify:
        raise SystemExit("GitHub verification must not execute pull-request workflow code")
if github_verify_cfg.get("forbid_gitlab_runner_labels", False):
    if "aigw-release-macos-arm64" in github_verify or "aigw-github-release-macos-arm64" in github_verify:
        raise SystemExit("GitHub verification must use only its dedicated runner label")
if github_verify_cfg.get("forbid_goproxy_policy", False):
    if "AIGW_GOPROXY" in github_verify or "goproxy.cn" in github_verify:
        raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if github_verify_cfg.get("forbid_write_permissions", False):
    if "pull-requests: write" in github_verify or "contents: write" in github_verify:
        raise SystemExit("verification workflow must use read-only repository permissions")
if github_verify_cfg.get("forbid_floating_actions", False):
    if "@main" in github_verify or "@master" in github_verify:
        raise SystemExit("GitHub Actions must use immutable action revisions")
if "go-version-file:" in github_verify or "check-latest: true" in github_verify:
    raise SystemExit("GitHub Actions verification must not float its Go toolchain")
if toolchain.get("github_setup_go_cache") is False and "cache: false" not in github_verify:
    raise SystemExit("GitHub verify must disable setup-go cache")

# Native acceptance projections on GitHub verify
require_tokens(github_verify, list(native_linux["required"]), "GitHub verify native Linux")
require_tokens(github_verify, list(native_windows["required"]), "GitHub verify native Windows")
require_tokens(github_verify, list(native_macos["staging_commands"]), "GitHub verify macOS staging")

# --- GitHub independent release plane ---
require_tokens(
    github_release,
    [
        f"name: {github_release_cfg['name']}",
        github_release_cfg["tag_filter"],
        f"permissions:\n  {github_release_cfg['permissions']}",
        github_release_cfg["needs"],
        f"runs-on: {github_release_cfg['runner']}",
        expected_github_go,
        f"GOTOOLCHAIN: {expected_gotoolchain}",
        "check-latest: false",
        "cache: false",
    ]
    + github_release_commands
    + list(native_linux["required"][:2])
    + list(native_windows["required"][:2]),
    "GitHub independent release plane",
)
for forbidden in github_release_forbid:
    if forbidden in github_release:
        raise SystemExit(f"GitHub release plane retains provider-specific build metadata: {forbidden}")
if "gitlab-ci" in github_release.lower() or re.search(
    r"(?m)^\s*sh scripts/release/publish/publish-gitlab-release\.sh(?:\s|$)", github_release
):
    raise SystemExit("GitHub release plane retains a non-peer dependency")
if "go-version-file:" in github_release or "check-latest: true" in github_release:
    raise SystemExit("GitHub release retains floating Go setup")

# GitLab must not claim GitHub-hosted native jobs.
for token in ("native-linux:", "native-windows:"):
    if token in gitlab:
        raise SystemExit(f"GitLab must not define GitHub-hosted native job {token}")

print("dual forge CI/CD contract: OK")
PYTHON
