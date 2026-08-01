#!/usr/bin/env python3
"""Validate and emit explicit release-time AIGW Forge coordinates."""

from __future__ import annotations

import argparse
import os
import re
import shlex
from pathlib import Path
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
DEFAULT_MANIFEST = ROOT / ".config" / "release" / "forge-sources.env"
KEYS = (
    "AIGW_GITLAB_RELEASE_ORIGIN",
    "AIGW_GITLAB_RELEASE_REPOSITORY",
    "AIGW_GITHUB_RELEASE_ORIGIN",
    "AIGW_GITHUB_RELEASE_REPOSITORY",
)
ASSIGNMENT = re.compile(r"^([A-Z0-9_]+)=([^\s#]+)$")
REPOSITORY = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]*(?:/[A-Za-z0-9][A-Za-z0-9._-]*)+$")


def fail(message: str) -> None:
    raise SystemExit(f"release forge-source manifest: {message}")


def load(path: Path) -> dict[str, str]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        fail(f"cannot read {path}: {exc}")

    values: dict[str, str] = {}
    for number, line in enumerate(lines, 1):
        if not line or line.startswith("#"):
            continue
        match = ASSIGNMENT.fullmatch(line)
        if match is None:
            fail(f"line {number} is not a plain KEY=value assignment")
        key, value = match.groups()
        if key not in KEYS:
            fail(f"line {number} has an unknown key: {key}")
        if key in values:
            fail(f"must define {key} exactly once")
        values[key] = value

    missing = [key for key in KEYS if key not in values]
    if missing:
        fail(f"must define {missing[0]} exactly once")
    return values


def validate_origin(key: str, value: str, *, https_only: bool) -> str:
    parsed = urlsplit(value)
    allowed = {"https"} if https_only else {"http", "https"}
    if parsed.scheme not in allowed or not parsed.netloc:
        protocol = "an HTTPS" if https_only else "an HTTP(S)"
        fail(f"{key} must be {protocol} origin")
    if parsed.username or parsed.password or parsed.path not in {"", "/"} or parsed.query or parsed.fragment:
        fail(f"{key} must not include credentials, path, query, or fragment")
    return value.rstrip("/")


def validate_repository(key: str, value: str) -> str:
    if not REPOSITORY.fullmatch(value):
        fail(f"{key} must be a slash-delimited repository path")
    return value


def resolve(path: Path) -> dict[str, str]:
    values = load(path)
    return {
        "AIGW_GITLAB_RELEASE_ORIGIN": validate_origin(
            "AIGW_GITLAB_RELEASE_ORIGIN", values["AIGW_GITLAB_RELEASE_ORIGIN"], https_only=False
        ),
        "AIGW_GITLAB_RELEASE_REPOSITORY": validate_repository(
            "AIGW_GITLAB_RELEASE_REPOSITORY", values["AIGW_GITLAB_RELEASE_REPOSITORY"]
        ),
        "AIGW_GITHUB_RELEASE_ORIGIN": validate_origin(
            "AIGW_GITHUB_RELEASE_ORIGIN", values["AIGW_GITHUB_RELEASE_ORIGIN"], https_only=True
        ),
        "AIGW_GITHUB_RELEASE_REPOSITORY": validate_repository(
            "AIGW_GITHUB_RELEASE_REPOSITORY", values["AIGW_GITHUB_RELEASE_REPOSITORY"]
        ),
    }


def resolve_environment() -> dict[str, str]:
    values = {key: os.environ.get(key, "").strip() for key in KEYS}
    missing = [key for key, value in values.items() if not value]
    if missing:
        fail(f"execution environment must define {missing[0]}")
    return {
        "AIGW_GITLAB_RELEASE_ORIGIN": validate_origin(
            "AIGW_GITLAB_RELEASE_ORIGIN", values["AIGW_GITLAB_RELEASE_ORIGIN"], https_only=False
        ),
        "AIGW_GITLAB_RELEASE_REPOSITORY": validate_repository(
            "AIGW_GITLAB_RELEASE_REPOSITORY", values["AIGW_GITLAB_RELEASE_REPOSITORY"]
        ),
        "AIGW_GITHUB_RELEASE_ORIGIN": validate_origin(
            "AIGW_GITHUB_RELEASE_ORIGIN", values["AIGW_GITHUB_RELEASE_ORIGIN"], https_only=True
        ),
        "AIGW_GITHUB_RELEASE_REPOSITORY": validate_repository(
            "AIGW_GITHUB_RELEASE_REPOSITORY", values["AIGW_GITHUB_RELEASE_REPOSITORY"]
        ),
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--file",
        type=Path,
        default=Path(os.environ.get("AIGW_RELEASE_FORGE_SOURCES_FILE", DEFAULT_MANIFEST)),
    )
    parser.add_argument("--shell", action="store_true", help="emit POSIX-safe export statements")
    parser.add_argument("--environment", action="store_true", help="read coordinates from the execution environment")
    args = parser.parse_args()
    values = resolve_environment() if args.environment else resolve(args.file)
    if args.shell:
        for key in KEYS:
            print(f"export {key}={shlex.quote(values[key])}")
        return
    for key in KEYS:
        print(f"{key}={values[key]}")


if __name__ == "__main__":
    main()
