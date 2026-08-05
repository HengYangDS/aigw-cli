#!/usr/bin/env python3
"""Verify one claim-bound quantitative observation against Git truth."""

from __future__ import annotations

import argparse
import hashlib
import re
import subprocess
import tomllib
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path, PurePosixPath


OBSERVATION = re.compile(
    r"Source-bound race coverage: commit `(?P<head>[0-9a-f]{40,64})`, "
    r"tree `(?P<tree>[0-9a-f]{40,64})`, `(?P<covered>[0-9]+)/"
    r"(?P<total>[0-9]+) = (?P<percent>[0-9]+\.[0-9]{2})%`; command "
    r"`go run \./tools/coveragegate --race`\."
)


def git(root: Path, *args: str) -> str:
    """Return one Git fact without consulting a shell."""

    result = subprocess.run(
        ["git", *args], cwd=root, check=False, text=True, capture_output=True
    )
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip() or "Git command failed"
        raise ValueError(detail)
    return result.stdout.strip()


def commit_tree(root: Path, commit: str) -> str | None:
    """Return a locally available commit's tree, or ``None`` for a peer-Forge ID."""

    result = subprocess.run(
        ["git", "cat-file", "-e", f"{commit}^{{commit}}"],
        cwd=root,
        check=False,
        text=True,
        capture_output=True,
    )
    if result.returncode != 0:
        return None
    return git(root, "rev-parse", f"{commit}^{{tree}}")


def tree_in_current_history(root: Path, expected: str) -> bool:
    """Return whether ``HEAD`` ancestry contains ``expected`` content."""

    return expected in git(root, "log", "--format=%T", "HEAD").splitlines()


def repository_path(root: Path, raw: object, label: str) -> Path:
    """Resolve one canonical repository-relative regular-file path."""

    if not isinstance(raw, str) or not raw:
        raise ValueError(f"{label} path is missing")
    relative = PurePosixPath(raw)
    if relative.is_absolute() or relative.as_posix() != raw or ".." in relative.parts:
        raise ValueError(f"{label} path is not repository-relative")
    path = root.joinpath(*relative.parts)
    if path.is_symlink() or not path.is_file():
        raise ValueError(f"{label} path is not a regular file")
    return path


def verify(root: Path, claim_path: Path) -> None:
    """Verify claim digest and its source-bound coverage observation."""

    root = root.resolve()
    claim_path = claim_path.resolve()
    try:
        claim_path.relative_to(root)
    except ValueError as exc:
        raise ValueError("claim path escapes repository") from exc
    with claim_path.open("rb") as stream:
        claim = tomllib.load(stream)
    evidence = claim.get("evidence")
    if not isinstance(evidence, dict):
        raise ValueError("claim evidence table is missing")
    record = repository_path(root, evidence.get("dated"), "dated evidence")
    expected_digest = evidence.get("sha256")
    actual_digest = hashlib.sha256(record.read_bytes()).hexdigest()
    if expected_digest != actual_digest:
        raise ValueError("evidence digest mismatch")

    matches = list(OBSERVATION.finditer(record.read_text(encoding="utf-8")))
    if len(matches) != 1:
        raise ValueError("dated evidence must contain exactly one source-bound coverage observation")
    observation = matches[0].groupdict()
    covered = int(observation["covered"])
    total = int(observation["total"])
    if total <= 0 or covered < 0 or covered > total:
        raise ValueError("coverage counts are invalid")
    derived = (Decimal(covered) * 100 / Decimal(total)).quantize(
        Decimal("0.01"), rounding=ROUND_HALF_UP
    )
    if derived != Decimal(observation["percent"]):
        raise ValueError("coverage percentage mismatch")
    actual_tree = commit_tree(root, observation["head"])
    if actual_tree is not None and actual_tree != observation["tree"]:
        raise ValueError("source tree mismatch")
    if not tree_in_current_history(root, observation["tree"]):
        raise ValueError("source tree is absent from current history")


def main() -> int:
    """Parse command arguments and report a bounded validation failure."""

    parser = argparse.ArgumentParser()
    parser.add_argument("claim", type=Path)
    parser.add_argument("--root", type=Path, default=Path.cwd())
    args = parser.parse_args()
    try:
        verify(args.root, args.root / args.claim)
    except (OSError, UnicodeError, ValueError, tomllib.TOMLDecodeError) as exc:
        parser.exit(1, f"evidence consistency: {exc}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
