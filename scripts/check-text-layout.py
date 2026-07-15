#!/usr/bin/env python3
"""Enforce portable, language-aware text-layout invariants for tracked source."""

from __future__ import annotations

import ast
from pathlib import Path
import subprocess
import sys
from typing import TypeAlias

ROOT = Path(__file__).resolve().parents[1]
PythonDeclaration: TypeAlias = ast.ClassDef | ast.FunctionDef | ast.AsyncFunctionDef
PYTHON_MODULE_DECLARATIONS: tuple[type[PythonDeclaration], ...] = (
    ast.ClassDef,
    ast.FunctionDef,
    ast.AsyncFunctionDef,
)
PYTHON_CLASS_DECLARATIONS: tuple[type[PythonDeclaration], ...] = (
    ast.FunctionDef,
    ast.AsyncFunctionDef,
)
TABLE_CONFIG_SUFFIXES = {".ini", ".toml"}


def tracked_files() -> list[Path]:
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\0")
    return [ROOT / name for name in names if name and (ROOT / name).is_file()]


def fail(path: Path, line: int, message: str) -> str:
    return f"{path.relative_to(ROOT)}:{line}: text layout: {message}"


def fenced_markdown(lines: list[str]) -> set[int]:
    protected: set[int] = set()
    active = False
    for number, line in enumerate(lines, 1):
        if line.lstrip().startswith(("```", "~~~")):
            active = not active
            protected.add(number)
        elif active:
            protected.add(number)
    return protected


def blank_lines_between(lines: list[str], left: int, right: int) -> int:
    return sum(1 for line in lines[left:right - 1] if not line.strip())


def check_config_table_boundaries(path: Path, lines: list[str]) -> list[str]:
    """Require one visual separator before each TOML or INI table."""
    if path.suffix.lower() not in TABLE_CONFIG_SUFFIXES:
        return []
    problems: list[str] = []
    for index, line in enumerate(lines):
        if not line.startswith("["):
            continue
        comment_start = index
        while comment_start > 0 and lines[comment_start - 1].lstrip().startswith(("#", ";")):
            comment_start -= 1
        if comment_start == index:
            if index > 0 and lines[index - 1].strip():
                problems.append(fail(path, index + 1, "use one blank line before a config table"))
        elif comment_start > 0 and lines[comment_start - 1].strip():
            problems.append(
                fail(path, comment_start + 1, "use one blank line before a config-table comment")
            )
    return problems


def check_python_compact_blocks(path: Path, lines: list[str]) -> list[str]:
    """Keep Python's two-line declaration rule out of function interiors."""
    try:
        tree = ast.parse("\n".join(lines), filename=str(path))
    except SyntaxError:
        return []
    problems: list[str] = []
    literal_lines = {
        line_number
        for node in ast.walk(tree)
        if isinstance(node, ast.Constant) and isinstance(node.value, str)
        for line_number in range(node.lineno, getattr(node, "end_lineno", node.lineno) + 1)
    }
    for node in ast.walk(tree):
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            continue
        body_start = min(
            (getattr(statement, "lineno", node.lineno) for statement in node.body),
            default=node.lineno,
        )
        end = getattr(node, "end_lineno", node.lineno)
        blank_start: int | None = None
        for number in range(body_start, end + 1):
            if number in literal_lines:
                blank_start = None
                continue
            if not lines[number - 1].strip():
                blank_start = number if blank_start is None else blank_start
                continue
            if blank_start is not None and number - blank_start > 1:
                problems.append(
                    fail(path, blank_start, "use at most one blank line inside a Python function")
                )
            blank_start = None
    return problems


def check_python_boundaries(path: Path, lines: list[str]) -> list[str]:
    try:
        tree = ast.parse("\n".join(lines), filename=str(path))
    except SyntaxError:
        return []
    problems: list[str] = []

    def check_declarations(
        nodes: list[ast.stmt],
        declaration_types: tuple[type[PythonDeclaration], ...],
        expected: int,
        scope: str,
    ) -> None:
        declarations = [node for node in nodes if isinstance(node, declaration_types)]
        for previous, current in zip(declarations, declarations[1:]):
            previous_end = getattr(previous, "end_lineno", previous.lineno)
            actual = blank_lines_between(lines, previous_end, current.lineno)
            if actual != expected:
                problems.append(
                    fail(
                        path,
                        previous_end + 1,
                        "use "
                        f"{expected} blank line{'s' if expected != 1 else ''} "
                        f"between {scope} declarations",
                    )
                )

    check_declarations(tree.body, PYTHON_MODULE_DECLARATIONS, 2, "module-level")
    for node in ast.walk(tree):
        if isinstance(node, ast.ClassDef):
            check_declarations(node.body, PYTHON_CLASS_DECLARATIONS, 1, "class-method")
    problems.extend(check_python_compact_blocks(path, lines))
    return problems


def inspect(path: Path) -> list[str]:
    data = path.read_bytes()
    if b"\0" in data:
        return []
    problems: list[str] = []
    if b"\r\n" in data or b"\r" in data:
        problems.append(fail(path, 1, "use LF line endings"))
    if not data.endswith(b"\n"):
        problems.append(fail(path, max(1, data.count(b"\n") + 1), "end with one newline"))
    try:
        lines = data.decode("utf-8").splitlines()
    except UnicodeDecodeError:
        return problems
    protected = fenced_markdown(lines) if path.suffix.lower() == ".md" else set()
    max_blank_run = 2 if path.suffix.lower() == ".py" else 1
    blank_start: int | None = None
    for number, line in enumerate(lines, 1):
        if line.rstrip(" \t") != line:
            problems.append(fail(path, number, "remove trailing whitespace"))
        if number in protected:
            blank_start = None
            continue
        if line.strip():
            if blank_start is not None and number - blank_start > max_blank_run:
                problems.append(
                    fail(
                        path,
                        blank_start,
                        "use at most "
                        f"{max_blank_run} consecutive blank line"
                        f"{'s' if max_blank_run != 1 else ''}",
                    )
                )
            blank_start = None
        elif blank_start is None:
            blank_start = number
    if blank_start is not None:
        problems.append(fail(path, blank_start, "use one final newline, not trailing blank lines"))
    if path.suffix.lower() == ".py":
        problems.extend(check_python_boundaries(path, lines))
    problems.extend(check_config_table_boundaries(path, lines))
    return problems


def main() -> None:
    problems = [problem for path in tracked_files() for problem in inspect(path)]
    if problems:
        print("\n".join(problems), file=sys.stderr)
        raise SystemExit(1)
    print("text layout contract: OK")


if __name__ == "__main__":
    main()
