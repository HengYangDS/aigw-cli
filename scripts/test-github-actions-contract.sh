#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
workflow="$root/.github/workflows/verify.yml"

[ -f "$workflow" ] || { echo "GitHub Actions verification workflow is missing" >&2; exit 1; }
python3 - "$workflow" <<'PYTHON'
from pathlib import Path
import re
import sys

text = Path(sys.argv[1]).read_text(encoding="utf-8")
required = [
    "name: Verify", "pull_request:", "push:", "workflow_dispatch:",
    "permissions:\n  contents: read",
    "runs-on: [self-hosted, macOS, ARM64, aigw-github-macos-arm64]",
    "actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd",
    "actions/setup-go@0c52d547c9bc32b1aa3301fd7a9cb496313a4491", 'go-version: "1.25.12"', "check-latest: false", "GOTOOLCHAIN: go1.25.12", "git fetch --force --tags origin", "if: github.ref_type == 'tag'", 'SELECTED_TAG: ${{ github.ref_name }}', 'scripts/check-release-tag-signature.sh . "$SELECTED_TAG" github', "scripts/check-release-toolchain.sh",
    "go test -race ./...", "go vet ./...", "scripts/check-static-analysis.sh", "scripts/check-product-surface.sh", "scripts/check-governance.sh",
    "scripts/check-text-layout.py", "scripts/test-text-layout.sh", "scripts/test-release-source-date-epoch.sh",
    "scripts/test-verified-candidate.sh", "scripts/test-release-tag-signature-provider-selection.sh", "scripts/test-macos-native-install-staging.sh",
    "shell: pwsh", "scripts/test-installers.ps1",
    "scripts/test-ci-go-cache-preparation.sh",
    "scripts/test-publish-release.sh", "scripts/test-publish-github-release.sh",
    "scripts/test-pipeline-gates.sh", "scripts/test-github-release-workflow.sh",
    "scripts/test-github-provider-projection.sh",
]
for token in required:
    if token not in text:
        raise SystemExit(f"GitHub Actions contract is missing {token!r}")
if "pull_request_target:" in text:
    raise SystemExit("GitHub verification must not use pull_request_target")
if "aigw-gitlab-macos-arm64" in text:
    raise SystemExit("GitHub verification must not use the GitLab runner label")
if "AIGW_GOPROXY" in text or "goproxy.cn" in text:
    raise SystemExit("GitHub Actions must not inherit GitLab-specific module proxy policy")
if "pull-requests: write" in text or "contents: write" in text:
    raise SystemExit("verification workflow must use read-only repository permissions")
if "@main" in text or "@master" in text:
    raise SystemExit("GitHub Actions must use immutable action revisions")
if "go-version-file:" in text or "check-latest: true" in text:
    raise SystemExit("GitHub Actions verification must not float its Go toolchain")
checkout = text.index("name: Check out full history and tags")
refresh = text.index("git fetch --force --tags origin")
provenance = text.index("name: Verify pushed release tag provenance")
gates = text.index("name: Run source and policy gates")
if not checkout < refresh < provenance < gates:
    raise SystemExit("GitHub Actions must refresh and verify annotated tags before source gates")
source_gate_commands = [
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

commented_job = text + "# low-indent comment\n    if: false\n"
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
