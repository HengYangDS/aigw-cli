#!/usr/bin/env python3
"""Regression tests for source-bound quantitative evidence."""

from __future__ import annotations

import hashlib
import importlib.util
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path


# Loading the checker is part of a repository gate, not a package installation.
# Keep that read-only validation from leaving bytecode beside governed source.
sys.dont_write_bytecode = True


ROOT = Path(__file__).resolve().parents[3]
CHECKER = ROOT / "scripts/checks/governance/check-evidence-consistency.py"


def load_checker():
    """Load the repository checker without adding a Python package facade."""

    spec = importlib.util.spec_from_file_location("evidence_consistency", CHECKER)
    if spec is None or spec.loader is None:
        raise RuntimeError("cannot load evidence consistency checker")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def git(root: Path, *args: str) -> str:
    """Run one isolated Git command and return trimmed standard output."""

    result = subprocess.run(
        ["git", *args], cwd=root, check=True, text=True, capture_output=True
    )
    return result.stdout.strip()


class EvidenceConsistencyTests(unittest.TestCase):
    """Exercise the quantitative observation and claim-digest boundary."""

    def test_loading_checker_leaves_no_repository_bytecode(self) -> None:
        cache = CHECKER.parent / "__pycache__"
        if cache.exists():
            self.fail(f"pre-existing repository bytecode residue: {cache}")
        load_checker()
        self.assertFalse(cache.exists())

    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        git(self.root, "init", "-q")
        git(self.root, "config", "user.name", "Test Actor")
        git(self.root, "config", "user.email", "test@example.invalid")
        (self.root / "source.txt").write_text("source\n", encoding="utf-8")
        git(self.root, "add", "source.txt")
        git(self.root, "commit", "-q", "-m", "test source")
        self.head = git(self.root, "rev-parse", "HEAD")
        self.tree = git(self.root, "rev-parse", "HEAD^{tree}")

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def write_record(
        self,
        *,
        head: str | None = None,
        tree: str | None = None,
        covered: int = 97,
        total: int = 100,
        percent: str = "97.00",
        observation: str | None = None,
    ) -> Path:
        """Write one dated record with a source-bound coverage observation."""

        record = self.root / "evidence/chronicle/test/2026-08-01.md"
        record.parent.mkdir(parents=True)
        line = observation or (
            "- Source-bound race coverage: "
            f"commit `{head or self.head}`, tree `{tree or self.tree}`, "
            f"`{covered}/{total} = {percent}%`; command "
            "`go run ./tools/coveragegate --race`."
        )
        record.write_text(f"# Test Evidence\n\n{line}\n", encoding="utf-8")
        return record

    def write_claim(self, record: Path, *, digest: str | None = None) -> Path:
        """Write the active claim binding for one record."""

        relative = record.relative_to(self.root).as_posix()
        actual = hashlib.sha256(record.read_bytes()).hexdigest()
        claim = self.root / "evidence/claims/test.toml"
        claim.parent.mkdir(parents=True)
        claim.write_text(
            textwrap.dedent(
                f"""
                [claim]
                id = "test"

                [evidence]
                dated = "{relative}"
                sha256 = "{digest or actual}"
                """
            ).lstrip(),
            encoding="utf-8",
        )
        return claim

    def test_valid_source_bound_observation_passes(self) -> None:
        checker = load_checker()
        record = self.write_record()
        checker.verify(self.root, self.write_claim(record))

    def test_peer_forge_commit_is_portable_when_tree_is_in_current_history(self) -> None:
        checker = load_checker()
        peer_commit = "0" * 40
        record = self.write_record(head=peer_commit)
        checker.verify(self.root, self.write_claim(record))

    def test_tree_absent_from_current_history_fails_closed(self) -> None:
        checker = load_checker()
        previous = self.head
        (self.root / "source.txt").write_text("successor\n", encoding="utf-8")
        git(self.root, "add", "source.txt")
        git(self.root, "commit", "-q", "-m", "successor")
        current_tree = git(self.root, "rev-parse", "HEAD^{tree}")
        git(self.root, "checkout", "-q", "--orphan", "independent")
        git(self.root, "rm", "-q", "-rf", ".")
        (self.root / "other.txt").write_text("independent\n", encoding="utf-8")
        git(self.root, "add", "other.txt")
        git(self.root, "commit", "-q", "-m", "independent")
        record = self.write_record(head="0" * 40, tree=current_tree)
        with self.assertRaisesRegex(ValueError, "source tree is absent from current history"):
            checker.verify(self.root, self.write_claim(record))

    def test_locally_available_commit_must_match_recorded_tree(self) -> None:
        checker = load_checker()
        record = self.write_record(head=self.head, tree="0" * 40)
        with self.assertRaisesRegex(ValueError, "source tree mismatch"):
            checker.verify(self.root, self.write_claim(record))

    def test_missing_raw_counts_fails_closed(self) -> None:
        checker = load_checker()
        record = self.write_record(
            observation=(
                "- Source-bound race coverage: "
                f"commit `{self.head}`, tree `{self.tree}`, `97.00%`."
            )
        )
        with self.assertRaisesRegex(ValueError, "source-bound coverage observation"):
            checker.verify(self.root, self.write_claim(record))

    def test_inconsistent_percentage_fails_closed(self) -> None:
        checker = load_checker()
        record = self.write_record(percent="96.99")
        with self.assertRaisesRegex(ValueError, "coverage percentage mismatch"):
            checker.verify(self.root, self.write_claim(record))

    def test_inconsistent_tree_fails_closed(self) -> None:
        checker = load_checker()
        record = self.write_record(tree="0" * 40)
        with self.assertRaisesRegex(ValueError, "source tree mismatch"):
            checker.verify(self.root, self.write_claim(record))

    def test_inconsistent_claim_digest_fails_closed(self) -> None:
        checker = load_checker()
        record = self.write_record()
        with self.assertRaisesRegex(ValueError, "evidence digest mismatch"):
            checker.verify(self.root, self.write_claim(record, digest="0" * 64))


if __name__ == "__main__":
    unittest.main()
