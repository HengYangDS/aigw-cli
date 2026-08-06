#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$root"

changelog=${AIGW_CHANGELOG_FILE:-CHANGELOG.md}
case "$changelog" in
  /*) ;;
  *) changelog="$root/$changelog" ;;
esac

selected_tag=${AIGW_CHANGELOG_RELEASE_TAG:-${CI_COMMIT_TAG:-}}
if test -z "$selected_tag" && test "${GITHUB_REF_TYPE:-}" = tag; then
  selected_tag=${GITHUB_REF_NAME:-}
fi

PYTHONDONTWRITEBYTECODE=1 python3 - "$changelog" "$selected_tag" <<'PY'
from __future__ import annotations

import datetime as dt
import re
import subprocess
import sys
from pathlib import Path

def fail(message: str) -> None:
    raise SystemExit(f"CHANGELOG.md: {message}")

path = Path(sys.argv[1])
selected_tag = sys.argv[2]
if not path.is_file():
    fail("missing file")

lines = path.read_text(encoding="utf-8").splitlines()
first_heading = next((line for line in lines if line.startswith("## ")), "")
if first_heading != "## [Unreleased]":
    fail("the first release section must be ## [Unreleased]")

heading = re.compile(r"^## \[([^]]+)] - (\d{4}-\d{2}-\d{2})$")
semver = re.compile(
    r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$"
)

def release_key(raw: str) -> tuple[object, ...]:
    match = semver.fullmatch(raw)
    if match is None:
        fail(f"invalid semantic version: {raw}")
    major, minor, patch, prerelease = match.groups()
    if prerelease is None:
        return int(major), int(minor), int(patch), 1, ()
    pieces = tuple(
        (0, int(piece)) if piece.isdigit() else (1, piece)
        for piece in prerelease.split(".")
    )
    return int(major), int(minor), int(patch), 0, pieces

entries: list[str] = []
for number, line in enumerate(lines, 1):
    if not line.startswith("## [") or line == "## [Unreleased]":
        continue
    match = heading.fullmatch(line)
    if match is None:
        fail(f"malformed published heading at line {number}: {line}")
    version, raw_date = match.groups()
    release_key(version)
    try:
        dt.date.fromisoformat(raw_date)
    except ValueError:
        fail(f"invalid release date at line {number}: {raw_date}")
    if version in entries:
        fail(f"duplicate published version at line {number}: {version}")
    entries.append(version)

if not entries:
    fail("missing published release heading")
if entries != sorted(entries, key=release_key, reverse=True):
    fail("published releases must appear once in strict descending semantic-version order")

if selected_tag:
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?", selected_tag):
        fail(f"selected release tag is malformed: {selected_tag}")
    selected_version = selected_tag.removeprefix("v")
    if entries[0] != selected_version:
        fail(f"first published section must identify selected release tag: {selected_tag}")
    try:
        tag_commit = subprocess.check_output(
            ["git", "rev-parse", f"refs/tags/{selected_tag}^{{}}"], text=True
        ).strip()
        head = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
    except subprocess.CalledProcessError:
        fail(f"selected release tag is unavailable: {selected_tag}")
    if tag_commit != head:
        fail(f"selected release tag does not identify HEAD: {selected_tag}")
PY
