#!/usr/bin/env python3
import sys
from pathlib import Path


if len(sys.argv) != 4:
    raise SystemExit("usage: compare-ordered-trees.py <canonical> <peer> <name>")

canonical = Path(sys.argv[1]).read_text(encoding="utf-8").splitlines()
peer = Path(sys.argv[2]).read_text(encoding="utf-8").splitlines()
name = sys.argv[3]

if canonical == peer:
    raise SystemExit(0)
if len(canonical) != len(peer):
    raise SystemExit(
        f"peer {name} does not preserve canonical ordered source-tree history: "
        f"expected {len(canonical)} entries, found {len(peer)}"
    )
for position, (expected, actual) in enumerate(zip(canonical, peer), 1):
    if expected != actual:
        raise SystemExit(
            f"peer {name} does not preserve canonical ordered source-tree history "
            f"at position {position}: expected {expected}, found {actual}"
        )
raise SystemExit(f"peer {name} does not preserve canonical ordered source-tree history")
