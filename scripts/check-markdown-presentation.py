#!/usr/bin/env python3
"""Reject Markdown metadata rows that would collapse in CommonMark renderers."""

from __future__ import annotations

import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ROOT_README = Path(os.environ.get("AIGW_README_FILE", ROOT / "README.md"))
PROJECT_NAME = "AIGW CLI"
PROJECT_PATH = "aigw-cli"

def fail(message: str) -> None:
    raise SystemExit(f"Markdown presentation contract: {message}")

def main() -> None:
    text = ROOT_README.read_text(encoding="utf-8")
    table = (
        "| GitLab metadata | Value |\n"
        "| --- | --- |\n"
        f"| **Project Name** | `{PROJECT_NAME}` |\n"
        f"| **Stable repository Path** | `{PROJECT_PATH}` |"
    )
    if table not in text:
        fail("README GitLab Project Name/Path must use one semantic table")
    if re.search(
        r"^\*\*GitLab Project Name:\*\*[^\n]*(?<!  )\n\*\*Stable repository Path:\*\*",
        text,
        flags=re.MULTILINE,
    ):
        fail("adjacent metadata rows would collapse without an explicit structure")
    print("Markdown presentation contract: OK")

if __name__ == "__main__":
    main()
