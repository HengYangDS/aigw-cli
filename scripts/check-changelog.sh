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

# The selected tag is a provider-local provenance object. The release
# chronicle is shared source metadata, so its date is validated syntactically
# rather than against either forge's independent tag timestamp.
selected_tag=${AIGW_CHANGELOG_RELEASE_TAG:-${CI_COMMIT_TAG:-}}
if test -z "$selected_tag" && test "${GITHUB_REF_TYPE:-}" = tag; then
  selected_tag=${GITHUB_REF_NAME:-}
fi
case "$selected_tag" in
  '') ;;
  v[0-9]*.*.*) ;;
  *) fail "selected release tag is malformed: $selected_tag" ;;
esac

forge=${AIGW_CHANGELOG_FORGE:-}
if test -z "$forge"; then
  if test "${GITHUB_ACTIONS:-}" = true; then
    forge=github
  elif test -n "${GITLAB_CI:-}"; then
    forge=gitlab
  else
    forge=local
  fi
fi
case "$forge" in
  gitlab|github|local) ;;
  *) fail "release forge is malformed: $forge" ;;
esac

# A selected tag can be a locally signed pre-push admission object. Validate it
# before any remote tag refresh, because origin legitimately does not advertise
# the new tag until the later, non-force push.
if test -n "$selected_tag"; then
  git rev-parse -q --verify "refs/tags/$selected_tag" >/dev/null || fail "selected release tag is unavailable: $selected_tag"
  test "$(git rev-parse "$selected_tag^{}")" = "$(git rev-parse HEAD)" || \
    fail "selected release tag does not identify HEAD: $selected_tag"
fi

has_origin=false
if git remote get-url origin >/dev/null 2>&1; then
  has_origin=true
  if test -z "$selected_tag"; then
    # A shallow or cached checkout can retain an older reachable tag while a
    # newer release tag is present only on origin. Refresh the complete tag
    # namespace before classifying shared Changelog history.
    git fetch --quiet --no-tags origin 'refs/tags/*:refs/tags/*' 2>/dev/null || true
  fi
fi

latest_tag=$selected_tag
if test -z "$latest_tag"; then
  latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
fi
if test -z "$latest_tag" && test "$has_origin" = true; then
  git fetch --quiet --no-tags --unshallow origin 2>/dev/null || \
    git fetch --quiet --no-tags origin 2>/dev/null || true
  if test -z "$selected_tag"; then
    latest_tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' HEAD 2>/dev/null || true)
  fi
fi
python3 - "$changelog" "$latest_tag" "$selected_tag" "$root/packaging/release/retired-gitlab-tags.txt" "$forge" <<'PYTHON'
from __future__ import annotations

import datetime as dt
import re
import subprocess
import sys
from pathlib import Path

path = Path(sys.argv[1])
latest_tag = sys.argv[2]
selected_tag = sys.argv[3]
retired_path = Path(sys.argv[4])
forge = sys.argv[5]
heading = re.compile(r"^## \[([^]]+)\] - (\d{4}-\d{2}-\d{2})$")
semver = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$")

def release_key(raw: str):
    match = semver.fullmatch(raw)
    if not match:
        raise ValueError(raw)
    major, minor, patch, prerelease = match.groups()
    if prerelease is None:
        return int(major), int(minor), int(patch), 1, ()
    pieces = []
    for piece in prerelease.split("."):
        pieces.append((0, int(piece)) if piece.isdigit() else (1, piece))
    return int(major), int(minor), int(patch), 0, tuple(pieces)

entries: list[str] = []
for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
    if not line.startswith("## [") or line == "## [Unreleased]":
        continue
    match = heading.fullmatch(line)
    if not match:
        raise SystemExit(f"CHANGELOG.md: malformed published heading at line {number}: {line}")
    version, raw_date = match.groups()
    try:
        release_key(version)
    except ValueError:
        raise SystemExit(f"CHANGELOG.md: invalid semantic version at line {number}: {version}")
    try:
        dt.date.fromisoformat(raw_date)
    except ValueError:
        raise SystemExit(f"CHANGELOG.md: invalid release date at line {number}: {raw_date}")
    if version in entries:
        raise SystemExit(f"CHANGELOG.md: duplicate published version at line {number}: {version}")
    entries.append(version)

if not entries:
    raise SystemExit("CHANGELOG.md: missing published release heading")

head_trees = set(subprocess.check_output(["git", "log", "HEAD", "--format=%T"], text=True).splitlines())
direct_tags = subprocess.check_output(["git", "tag", "--list", "v[0-9]*"], text=True).splitlines()
github_tags = subprocess.check_output(["git", "tag", "--list", "github/v[0-9]*"], text=True).splitlines()

# Canonical checkouts keep fetched GitHub release tags beneath `github/` so
# provider-native provenance never collides with GitLab's unscoped tag names.
# A native GitHub checkout has only the direct names. Local chronology may
# inspect both providers, but a GitLab-only check must never accept a GitHub tag
# merely because both providers release the same version.
if forge == "github":
    # A canonical checkout can locally audit the remote GitHub namespace under
    # `github/`; a native GitHub checkout exposes the same tags directly. Do
    # not combine both here: an unscoped GitLab tag must not masquerade as
    # GitHub provenance when qualified tags are available.
    tags = github_tags or direct_tags
elif forge == "local":
    # Local validation accepts every complete fetched provider namespace. The
    # source-tree identity below prevents a foreign GitHub tag from becoming
    # local chronology unless this branch actually contains its source tree.
    tags = direct_tags + github_tags
else:
    tags = direct_tags

tag_versions = []
tag_trees: dict[str, str] = {}
for tag in tags:
    version = tag.removeprefix("github/").removeprefix("v")
    try:
        release_key(version)
    except ValueError:
        continue
    peeled = subprocess.check_output(["git", "rev-parse", f"{tag}^{{}}"], text=True).strip()
    tree = subprocess.check_output(["git", "rev-parse", f"{peeled}^{{tree}}"], text=True).strip()
    if forge == "github":
        # GitHub's identity projection rewrites commits while preserving the
        # released source tree and its signed provider-native provenance tag.
        # A tag is active there only when its exact tree is represented by HEAD.
        active = tree in head_trees
    elif forge == "local":
        # Local validation accepts a complete fetched tag namespace so a
        # shallow checkout can recover chronology from its configured origin.
        active = True
    else:
        # Canonical GitLab/local history keeps its own identity. A foreign tag
        # copied into the local object store is not a canonical release merely
        # because it happens to name an equivalent tree.
        active = subprocess.run(
            ["git", "merge-base", "--is-ancestor", peeled, "HEAD"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        ).returncode == 0
    if not active:
        continue
    previous_tree = tag_trees.get(version)
    if previous_tree is not None:
        if previous_tree != tree:
            raise SystemExit(f"CHANGELOG.md: provider tags disagree on source tree for version {version}")
        continue
    tag_trees[version] = tree
    tag_versions.append(version)
if latest_tag:
    try:
        latest_version = latest_tag.removeprefix("v")
        release_key(latest_version)
    except ValueError:
        raise SystemExit(f"CHANGELOG.md: invalid latest release tag: {latest_tag}")
    if latest_version not in tag_versions:
        tag_versions.append(latest_version)
if not tag_versions:
    raise SystemExit("CHANGELOG.md: cannot find an active release tag for the selected forge")
tag_versions.sort(key=release_key, reverse=True)

retired_versions = []
if retired_path.exists():
    for number, line in enumerate(retired_path.read_text(encoding="utf-8").splitlines(), 1):
        raw = line.strip()
        if not raw or raw.startswith("#"):
            continue
        if not raw.startswith("v"):
            raise SystemExit(f"CHANGELOG.md: malformed retired tag inventory at line {number}: {raw}")
        version = raw.removeprefix("v")
        try:
            release_key(version)
        except ValueError:
            raise SystemExit(f"CHANGELOG.md: invalid retired tag inventory at line {number}: {raw}")
        if version in retired_versions:
            raise SystemExit(f"CHANGELOG.md: duplicate active or retired release version: {version}")
        if version in tag_versions:
            # The inventory records GitLab retirement. A same-named GitHub
            # provenance tag remains active and takes precedence on GitHub.
            if forge != "github":
                raise SystemExit(f"CHANGELOG.md: duplicate active or retired release version: {version}")
            continue
        retired_versions.append(version)

# All known tags must have exactly one heading in matching descending order.
# An untagged candidate is allowed only as the single first release section on
# an ordinary branch, so release preparation can pass before either forge signs
# its provider-native tag.
managed_versions = tag_versions + retired_versions
managed_versions.sort(key=release_key, reverse=True)
known = [version for version in entries if version in managed_versions]
if known != managed_versions:
    raise SystemExit("CHANGELOG.md: active and retired release tags must appear once in descending version order")
unknown = [version for version in entries if version not in managed_versions]

if selected_tag:
    selected_version = selected_tag.removeprefix("v")
    if entries[0] != selected_version:
        raise SystemExit(f"CHANGELOG.md: first published section must identify selected release tag: {selected_tag}")
    if unknown:
        raise SystemExit("CHANGELOG.md: selected tag pipeline contains an untagged release heading")
else:
    latest_version = tag_versions[0]
    if not unknown:
        if entries[0] != latest_version:
            raise SystemExit(f"CHANGELOG.md: first published section must identify the latest active tag: v{latest_version}")
    elif len(unknown) != 1 or entries[0] != unknown[0]:
        raise SystemExit("CHANGELOG.md: only the first release heading may be an untagged next candidate")
    elif release_key(unknown[0]) <= release_key(tag_versions[0]):
        raise SystemExit("CHANGELOG.md: next candidate must sort after the latest release tag")
PYTHON
