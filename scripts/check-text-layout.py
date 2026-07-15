#!/usr/bin/env python3
"""Enforce a compact, portable text-layout contract for tracked source files.

One empty line separates prose paragraphs, declarations, and configuration
blocks. Consecutive empty lines, whitespace-only empty lines, leading empty
lines, and terminal empty blocks are never semantic and are therefore rejected.
The only permitted exception is literal content inside a fenced Markdown block.
Language-specific structural spacing remains the formatter's authority:
Python uses PEP 8 two-line separation between module-level declarations, while
Go and configuration files retain their native grammar and style rules.
"""

from __future__ import annotations

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TEXT_NAMES = {"AGENTS.md", "CHANGELOG.md", "CONTRIBUTING.md", "README.md", "aigw"}
TEXT_SUFFIXES = {".go", ".json", ".md", ".ps1", ".py", ".sh", ".toml", ".txt", ".wxs", ".yaml", ".yml"}


def is_text_path(name: str) -> bool:
    path = Path(name)
    return path.name in TEXT_NAMES or path.suffix.lower() in TEXT_SUFFIXES


def main() -> None:
    tracked = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\0")
    failures: list[str] = []
    for name in filter(is_text_path, tracked):
        path = ROOT / name
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeDecodeError):
            continue
        fenced = False
        blank_run = 0
        for number, line in enumerate(lines, 1):
            if name.endswith(".md") and line.startswith("```"):
                fenced = not fenced
            if fenced:
                blank_run = 0
                continue
            if line and line[-1].isspace():
                failures.append(f"{name}:{number}: trailing whitespace")
            if line:
                blank_run = 0
                continue
            blank_run += 1
            allowed = 2 if name.endswith(".py") else 1
            if blank_run > allowed:
                failures.append(f"{name}:{number}: consecutive blank line")
        if lines and not lines[0]:
            failures.append(f"{name}:1: leading blank line")
        if lines and not lines[-1]:
            failures.append(f"{name}:{len(lines)}: terminal blank line")
    if failures:
        raise SystemExit("Text layout contract failed:\n" + "\n".join(failures))
    print("Text layout contract: OK")


if __name__ == "__main__":
    main()
