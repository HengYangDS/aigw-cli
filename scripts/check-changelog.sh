#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

fail() {
  echo "CHANGELOG.md: $*" >&2
  exit 1
}

changelog=${AIGW_CHANGELOG_FILE:-CHANGELOG.md}
case "$changelog" in
  /*) ;;
  *) changelog="$root/$changelog" ;;
esac
test -f "$changelog" || fail "missing file"

first_heading=$(awk '/^## / { print; exit }' "$changelog")
test "$first_heading" = "## [Unreleased]" || \
  fail "the first release section must be ## [Unreleased]"

# A release pipeline validates its selected tag, not an arbitrary older tag
# reachable from its checkout. CI providers expose the selected tag through
# different variables; the explicit AIGW variable supports local rehearsal.
selected_tag=${AIGW_CHANGELOG_RELEASE_TAG:-${CI_COMMIT_TAG:-}}
if test -z "$selected_tag" && test "${GITHUB_REF_TYPE:-}" = tag; then
  selected_tag=${GITHUB_REF_NAME:-}
fi
case "$selected_tag" in
  '') ;;
  v[0-9]*.*.*) ;;
  *) fail "selected release tag is malformed: $selected_tag" ;;
esac

# A shallow checkout can omit historical refs. Refresh once before judging a
# branch chronology; an unavailable selected tag remains a hard failure.
latest_tag=$selected_tag
if test -z "$latest_tag"; then
  latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
fi
if { test -z "$latest_tag" || ! git rev-parse -q --verify "refs/tags/$latest_tag" >/dev/null 2>&1; } && git remote get-url origin >/dev/null 2>&1; then
  git fetch --quiet --no-tags --unshallow origin 2>/dev/null || \
    git fetch --quiet --no-tags origin 2>/dev/null || true
  git fetch --quiet --no-tags origin 'refs/tags/*:refs/tags/*' 2>/dev/null || true
  if test -z "$selected_tag"; then
    latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
  fi
fi
if test -n "$selected_tag"; then
  git rev-parse -q --verify "refs/tags/$selected_tag" >/dev/null || fail "selected release tag is unavailable: $selected_tag"
  test "$(git rev-parse "$selected_tag^{}")" = "$(git rev-parse HEAD)" || \
    fail "selected release tag does not identify HEAD: $selected_tag"
elif test -z "$latest_tag"; then
  fail "cannot find a reachable v<semver> Git tag"
fi

# A branch may carry exactly one next release heading before its independent
# GitLab and GitHub provenance tags exist. A tag pipeline cannot: it must name
# the first heading exactly. This lets both forge planes validate the identical
# source tree without treating one provider's tag timestamp as shared state.
python3 - "$changelog" "$selected_tag" "$(git rev-parse --is-shallow-repository)" <<'PYTHON'
from __future__ import annotations

import datetime as dt
import re
import subprocess
import sys
from pathlib import Path

path = Path(sys.argv[1])
selected_tag = sys.argv[2]
is_shallow = sys.argv[3] == "true"
heading = re.compile(r"^## \[([^]]+)\] - (\d{4}-\d{2}-\d{2})$")
semver = re.compile(r"^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$")

def parse_version(raw: str) -> tuple[int, int, int, tuple[tuple[int, object], ...]]:
    match = semver.fullmatch(raw)
    if not match:
        raise ValueError(raw)
    major, minor, patch, prerelease = match.groups()
    if prerelease is None:
        suffix: tuple[tuple[int, object], ...] = ((2, ""),)
    else:
        items: list[tuple[int, object]] = []
        for item in prerelease.split("."):
            if item.isdigit():
                items.append((0, int(item)))
            else:
                items.append((1, item))
        suffix = tuple(items)
    return int(major), int(minor), int(patch), suffix

def sort_key(raw: str):
    return parse_version(raw)

versions: list[str] = []
for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
    if not line.startswith("## [") or line == "## [Unreleased]":
        continue
    match = heading.fullmatch(line)
    if not match:
        raise SystemExit(f"CHANGELOG.md: malformed published heading at line {number}: {line}")
    version, raw_date = match.groups()
    try:
        parse_version(version)
    except ValueError:
        raise SystemExit(f"CHANGELOG.md: invalid semantic version at line {number}: {version}")
    try:
        dt.date.fromisoformat(raw_date)
    except ValueError:
        raise SystemExit(f"CHANGELOG.md: invalid release date at line {number}: {raw_date}")
    if version in versions:
        raise SystemExit(f"CHANGELOG.md: duplicate published version at line {number}: {version}")
    versions.append(version)

if not versions:
    raise SystemExit("CHANGELOG.md: missing published release heading")
tags = subprocess.check_output(
    ["git", "tag", "--list", "v[0-9]*"], text=True
).splitlines()
tag_versions = []
for tag in tags:
    raw = tag.removeprefix("v")
    try:
        parse_version(raw)
    except ValueError:
        continue
    tag_versions.append(raw)
if not tag_versions:
    raise SystemExit("CHANGELOG.md: cannot find SemVer release tags")
tag_versions.sort(key=sort_key, reverse=True)

if selected_tag:
    selected_version = selected_tag.removeprefix("v")
    if versions[0] != selected_version:
        raise SystemExit(
            f"CHANGELOG.md: first published section must identify selected release tag: {selected_tag}"
        )

known = [version for version in versions if version in tag_versions]
if known != tag_versions:
    raise SystemExit("CHANGELOG.md: locally available release tags must appear once in descending version order")

if is_shallow:
    raise SystemExit(0)

unknown = [version for version in versions if version not in tag_versions]
if selected_tag:
    if unknown:
        raise SystemExit("CHANGELOG.md: selected tag pipeline contains an untagged release heading")
elif unknown:
    if len(unknown) != 1 or versions[0] != unknown[0]:
        raise SystemExit("CHANGELOG.md: only the first release heading may be an untagged next candidate")
    if sort_key(unknown[0]) <= sort_key(tag_versions[0]):
        raise SystemExit("CHANGELOG.md: next candidate must sort after the latest reachable release tag")
elif versions[0] != tag_versions[0]:
    raise SystemExit("CHANGELOG.md: first published section must identify the latest reachable release tag")
PYTHON
