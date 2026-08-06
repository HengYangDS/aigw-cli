#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
workflow="$root/.github/workflows/verify.yml"

[ -f "$workflow" ] || { echo "GitHub Actions verification workflow is missing" >&2; exit 1; }
python3 - "$workflow" "$root/go.mod" "$root/.config/ci/verify-gates.toml" <<'PYTHON'
from pathlib import Path
import re
import tomllib
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
go_version = next(line.split()[1] for line in Path(sys.argv[2]).read_text(encoding="utf-8").splitlines() if line.startswith("go "))
actions = tomllib.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))["toolchain"]["github_actions"]
checkout_action = actions["checkout"]
setup_go_action = actions["setup_go"]
for key, expected_name in (("checkout", "actions/checkout"), ("setup_go", "actions/setup-go")):
    value = actions[key]
    if re.fullmatch(rf"{re.escape(expected_name)}@[0-9a-f]{{40}}", value) is None:
        raise SystemExit(f"GitHub Action {key} must pin {expected_name} by a 40-character commit SHA")
required = [
    "name: Verify", "push:", "workflow_dispatch:", "branches: [main]", "tags: ['v*']",
    "permissions:\n  contents: read",
    "runs-on: ${{ fromJSON(vars.AIGW_VERIFY_RUNNER) }}",
    checkout_action,
    setup_go_action, "go-version-file: go.mod", "check-latest: false", "cache: false", "for attempt in 1 2 3; do", "if git fetch --force --tags origin; then", 'sleep "$attempt"', "if: github.ref_type == 'tag'", 'SELECTED_TAG: ${{ github.ref_name }}', 'scripts/checks/forge/check-release-tag-signature.sh . "$SELECTED_TAG" github', "scripts/checks/release/check-release-toolchain.sh",
    "go run ./tools/architecture --root .", "go run ./tools/coveragegate --race", "go vet ./...", "scripts/checks/quality/check-static-analysis.sh", "for script in $(git ls-files 'scripts/*.sh'); do sh -n \"$script\"; done", "scripts/checks/governance/check-product-surface.sh", "scripts/checks/governance/check-governance.sh",
    "scripts/checks/forge/check-commit-provenance.sh . github", "scripts/tests/forge/test-commit-provenance.sh", "scripts/tests/forge/test-replay-history.py", "AIGW_CHANGELOG_RELEASE_TAG:",
    "scripts/checks/governance/check-text-layout.py", "scripts/tests/governance/test-text-layout.sh", "scripts/tests/release/test-release-source-date-epoch.sh",
    "scripts/tests/release/test-verified-candidate.sh", "scripts/tests/forge/test-release-tag-signature-provider-selection.sh", "scripts/tests/install/test-macos-native-install-staging.sh",
    "shell: pwsh", "scripts/tests/install/test-installers.ps1",
    "scripts/tests/ci/test-ci-go-cache-preparation.sh",
    "scripts/tests/release/test-publish-gitlab-release.sh", "scripts/tests/release/test-publish-github-release.sh",
    "scripts/tests/ci/test-pipeline-gates.sh", "scripts/tests/ci/test-github-release-workflow.sh",
    "scripts/tests/forge/test-github-provider-projection.sh", "scripts/tests/forge/test-forge-sync.sh",
    "native-linux:", "runs-on: ubuntu-latest", "native-windows:", "runs-on: windows-latest",
    "name: Materialize provenance trust input", "AIGW_RELEASE_ALLOWED_SIGNERS: ${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}",
    'allowed_signers="$RUNNER_TEMP/aigw-allowed-signers"', 'printf \'%s\\n\' "$AIGW_RELEASE_ALLOWED_SIGNERS" > "$allowed_signers"',
    'echo "AIGW_GITHUB_ALLOWED_SIGNERS=$allowed_signers" >> "$GITHUB_ENV"',
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions contract is missing {token!r}")
if "pull_request:" in text or "pull_request_target:" in text:
    raise SystemExit("GitHub verification must not execute pull-request workflow code")
if "runs-on: [self-hosted" in text or "aigw-github-verify-macos-arm64" in text:
    raise SystemExit("GitHub verification hardcodes adopter runner inventory")
if "AIGW_GOPROXY" in text or "goproxy.cn" in text:
    raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if "AIGW_GITHUB_ALLOWED_SIGNERS: ${{ vars.AIGW_RELEASE_ALLOWED_SIGNERS }}" in text:
    raise SystemExit("GitHub Actions must not pass trust content where a checker requires a path")
if text.count("name: Materialize provenance trust input") != 2:
    raise SystemExit("GitHub verification must materialize trust input once per provenance job")
if "pull-requests: write" in text or "contents: write" in text:
    raise SystemExit("verification workflow must use read-only repository permissions")
if "@main" in text or "@master" in text:
    raise SystemExit("GitHub Actions must use immutable action revisions")
if f'go-version: "{go_version}"' in text or f"GOTOOLCHAIN: go{go_version}" in text:
    raise SystemExit("GitHub Actions verification duplicates the project Go version")
if "check-latest: true" in text:
    raise SystemExit("GitHub Actions verification must not request a newer Go toolchain")
setup = text.index(setup_go_action)
cache = text.index("cache: false", setup)
gates = text.index("name: Run source and policy gates")
if not setup < cache < gates:
    raise SystemExit("GitHub Actions verification must disable setup-go cache before source gates")
checkout = text.index("name: Check out full history and tags")
refresh = text.index("for attempt in 1 2 3; do")
fetch = text.index("git fetch --force --tags origin", refresh)
retry_wait = text.index('sleep "$attempt"', fetch)
provenance = text.index("name: Verify pushed release tag provenance")
if not checkout < refresh < fetch < retry_wait < provenance < gates:
    raise SystemExit("GitHub Actions must refresh and verify annotated tags before source gates")
source_gate_commands = [
    "sh scripts/checks/security/check-credential-literals.sh",
    "sh scripts/tests/security/test-credential-literals.sh",
    "sh scripts/checks/security/check-credential-fixtures.sh",
    "sh scripts/tests/security/test-credential-fixtures.sh",
    "sh scripts/tests/forge/test-branch-closeout.sh",
    "sh scripts/tests/forge/test-forge-sync.sh",
]

def check_canonical_mapping_keys(lines, indent, banned, context):
    for line in lines:
        if len(line) - len(line.lstrip()) != indent:
            continue
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        match = re.match(r"^([A-Za-z0-9_-]+)\s*:", stripped)
        if match is None:
            raise SystemExit(f"{context} contains non-canonical mapping key syntax: {stripped}")
        key = match.group(1)
        if key in banned:
            raise SystemExit(f"{context} must not define {key}")

def check_source_gate_commands(workflow: str) -> None:
    lines = workflow.splitlines()
    jobs_start = next(index for index, line in enumerate(lines) if line == "jobs:")
    job_start = next(
        index for index in range(jobs_start + 1, len(lines)) if lines[index] == "  verify:"
    )
    job_indent = len(lines[job_start]) - len(lines[job_start].lstrip())
    job_end = next(
        (
            index
            for index in range(job_start + 1, len(lines))
            if lines[index].strip()
            and not lines[index].lstrip().startswith("#")
            and len(lines[index]) - len(lines[index].lstrip()) <= job_indent
        ),
        len(lines),
    )
    check_canonical_mapping_keys(
        lines[job_start + 1:job_end],
        job_indent + 2,
        {"continue-on-error", "if"},
        "GitHub Actions verify job",
    )
    step_start = next(
        index
        for index in range(job_start + 1, job_end)
        if lines[index].strip() == "- name: Run source and policy gates"
    )
    step_indent = len(lines[step_start]) - len(lines[step_start].lstrip())
    step_end = next(
        (
            index
            for index in range(step_start + 1, job_end)
            if lines[index].strip()
            and not lines[index].lstrip().startswith("#")
            and len(lines[index]) - len(lines[index].lstrip()) == step_indent
            and lines[index].lstrip().startswith("- ")
        ),
        job_end,
    )
    check_canonical_mapping_keys(
        lines[step_start + 1:step_end],
        step_indent + 2,
        {"continue-on-error", "if"},
        "GitHub Actions source gate step",
    )
    run_start = next(
        index
        for index in range(step_start + 1, step_end)
        if lines[index].strip() == "run: |"
        and len(lines[index]) - len(lines[index].lstrip()) == step_indent + 2
    )
    run_indent = len(lines[run_start]) - len(lines[run_start].lstrip())
    run_end = next(
        (
            index
            for index in range(run_start + 1, step_end)
            if lines[index].strip()
            and not lines[index].lstrip().startswith("#")
            and len(lines[index]) - len(lines[index].lstrip()) <= run_indent
        ),
        step_end,
    )
    run_block = "\n".join(lines[run_start + 1:run_end])
    for command in source_gate_commands:
        if not re.search(rf"(?m)^[ \t]+{re.escape(command)}[ \t]*$", run_block):
            raise SystemExit(f"GitHub Actions source gates are missing active command: {command}")

check_source_gate_commands(text)

def expect_nonblocking_rejection(workflow: str, expected: str, fixture: str) -> None:
    try:
        check_source_gate_commands(workflow)
    except SystemExit as error:
        if str(error) != expected:
            raise SystemExit(f"GitHub Actions {fixture} failed unexpectedly: {error}")
    else:
        raise SystemExit(f"GitHub Actions contract accepted {fixture}")

gate_start = text.index("      - name: Run source and policy gates\n")
run_start = text.index("        run: |\n", gate_start)
job_insert = text.index("  verify:\n") + len("  verify:\n")
for position, line, expected, fixture in [
    (run_start, "        continue-on-error: true\n", "GitHub Actions source gate step must not define continue-on-error", "step continue-on-error"),
    (run_start, "        if: false\n", "GitHub Actions source gate step must not define if", "step if"),
    (run_start, '        "\\u0069f": false\n', 'GitHub Actions source gate step contains non-canonical mapping key syntax: "\\u0069f": false', "escaped step if"),
    (run_start, '        "if" : false\n', 'GitHub Actions source gate step contains non-canonical mapping key syntax: "if" : false', "quoted step if"),
    (job_insert, "    continue-on-error: true\n", "GitHub Actions verify job must not define continue-on-error", "job continue-on-error"),
    (job_insert, "    if: false\n", "GitHub Actions verify job must not define if", "job if"),
]:
    expect_nonblocking_rejection(text[:position] + line + text[position:], expected, fixture)

next_step = text.index("\n      - name: Run PowerShell installer contract", run_start)
commented_step = text[:next_step] + "\n# low-indent comment\n        if: false" + text[next_step:]
expect_nonblocking_rejection(
    commented_step,
    "GitHub Actions source gate step must not define if",
    "step if after a low-indent comment",
)

next_job = text.index("\n  native-linux:")
commented_job = text[:next_job] + "\n# low-indent comment\n    if: false" + text[next_job:]
expect_nonblocking_rejection(
    commented_job,
    "GitHub Actions verify job must not define if",
    "job if after a low-indent comment",
)

# Exact command text inside step environment data is inert and must not satisfy
# the source-gate execution contract.
inactive_command = source_gate_commands[0]
inactive_line = f"          {inactive_command}\n"
if text.count(inactive_line) != 1:
    raise SystemExit("GitHub Actions active-command fixture has an unexpected command count")
inactive = text.replace(inactive_line, "", 1)
gate_start = inactive.index("      - name: Run source and policy gates\n")
run_start = inactive.index("        run: |\n", gate_start)
inactive_manifest = (
    "          INACTIVE_GATE_MANIFEST: |\n"
    f"            {inactive_command}\n"
)
inactive = inactive[:run_start] + inactive_manifest + inactive[run_start:]
try:
    check_source_gate_commands(inactive)
except SystemExit as error:
    expected = f"GitHub Actions source gates are missing active command: {inactive_command}"
    if str(error) != expected:
        raise SystemExit(f"GitHub Actions env-data fixture failed for an unexpected reason: {error}")
else:
    raise SystemExit("GitHub Actions contract accepted a source gate present only in step env data")

# A block scalar inside env can contain text that resembles a nested run key.
# Neither that fake key nor its command manifest is an executable step property.
nested_run = text
for command in source_gate_commands:
    command_line = f"          {command}\n"
    if nested_run.count(command_line) != 1:
        raise SystemExit("GitHub Actions nested-run fixture has an unexpected command count")
    nested_run = nested_run.replace(command_line, "", 1)
gate_start = nested_run.index("      - name: Run source and policy gates\n")
run_start = nested_run.index("        run: |\n", gate_start)
nested_manifest = (
    "          INACTIVE_NESTED_RUN: |\n"
    "            run: |\n"
    + "".join(f"              {command}\n" for command in source_gate_commands)
)
nested_run = nested_run[:run_start] + nested_manifest + nested_run[run_start:]
try:
    check_source_gate_commands(nested_run)
except SystemExit as error:
    expected = f"GitHub Actions source gates are missing active command: {source_gate_commands[0]}"
    if str(error) != expected:
        raise SystemExit(f"GitHub Actions nested-run fixture failed for an unexpected reason: {error}")
else:
    raise SystemExit("GitHub Actions contract accepted commands under a fake run key in step env data")
print("GitHub Actions verification contract: OK")
PYTHON
