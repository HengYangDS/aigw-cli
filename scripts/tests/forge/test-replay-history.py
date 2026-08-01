#!/usr/bin/env python3
"""Regression tests for byte-exact Forge history replay."""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
REPLAY = ROOT / "scripts/forge/lib/replay-history.py"


def run(*args: str, cwd: Path | None = None, input_bytes: bytes | None = None) -> bytes:
    """Run one isolated command and return standard output bytes."""

    result = subprocess.run(
        args,
        cwd=cwd,
        input=input_bytes,
        check=True,
        capture_output=True,
        env={
            **os.environ,
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_NOSYSTEM": "1",
            "PYTHONDONTWRITEBYTECODE": "1",
        },
    )
    return result.stdout


def git(repository: Path, *args: str, input_bytes: bytes | None = None) -> bytes:
    """Run Git against one repository."""

    return run("git", "-C", str(repository), *args, input_bytes=input_bytes)


def commit_message(repository: Path, revision: str) -> bytes:
    """Return the raw message section from one commit object."""

    return git(repository, "cat-file", "commit", revision).split(b"\n\n", 1)[1]


class ReplayHistoryTests(unittest.TestCase):
    """Prove semantic preservation and target-object isolation."""

    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.source = self.root / "source"
        self.output = self.root / "replayed.git"
        self.key = self.root / "signing"
        self.allowed = self.root / "allowed-signers"
        run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(self.key))
        public = (self.key.with_suffix(".pub")).read_text(encoding="utf-8").split()
        self.allowed.write_text(
            f'forge@example.invalid namespaces="git" {public[0]} {public[1]}\n',
            encoding="utf-8",
        )
        run("git", "init", "-q", "-b", "main", str(self.source))
        git(self.source, "config", "user.name", "Source Fixture")
        git(self.source, "config", "user.email", "source@example.invalid")
        git(self.source, "config", "commit.gpgsign", "false")
        (self.source / "source.txt").write_text("root\n", encoding="utf-8")
        git(self.source, "add", "source.txt")
        git(self.source, "commit", "-q", "-m", "root")

    def tearDown(self) -> None:
        self.temporary.cleanup()

    def raw_commit(
        self,
        message: bytes,
        *parents: str,
        extra_headers: tuple[bytes, ...] = (),
    ) -> str:
        """Create one source commit with an exact raw message."""

        tree = git(self.source, "rev-parse", "HEAD^{tree}").decode().strip()
        if extra_headers:
            parent_headers = b"".join(
                f"parent {parent}\n".encode("ascii") for parent in parents
            )
            identity = b"Source Fixture <source@example.invalid>"
            raw = b"".join(
                (
                    f"tree {tree}\n".encode("ascii"),
                    parent_headers,
                    b"author " + identity + b" 1700000001 +0530\n",
                    b"committer " + identity + b" 1700000002 -0700\n",
                    b"".join(header + b"\n" for header in extra_headers),
                    b"\n",
                    message,
                )
            )
            return git(self.source, "hash-object", "-t", "commit", "-w", "--stdin", input_bytes=raw).decode().strip()
        command = ["commit-tree", tree]
        for parent in parents:
            command.extend(("-p", parent))
        env = {
            **os.environ,
            "GIT_AUTHOR_NAME": "Source Fixture",
            "GIT_AUTHOR_EMAIL": "source@example.invalid",
            "GIT_AUTHOR_DATE": "1700000001 +0530",
            "GIT_COMMITTER_NAME": "Source Fixture",
            "GIT_COMMITTER_EMAIL": "source@example.invalid",
            "GIT_COMMITTER_DATE": "1700000002 -0700",
            "GIT_CONFIG_GLOBAL": "/dev/null",
            "GIT_CONFIG_NOSYSTEM": "1",
        }
        result = subprocess.run(
            ["git", "-C", str(self.source), *command],
            input=message,
            check=True,
            capture_output=True,
            env=env,
        )
        return result.stdout.decode().strip()

    def replay(self, revision: str) -> dict[str, object]:
        """Replay the selected graph and return its receipt."""

        run(
            "python3",
            str(REPLAY),
            "--source",
            str(self.source),
            "--revision",
            revision,
            "--output",
            str(self.output),
            "--ref",
            "refs/heads/main",
            "--actor-name",
            "Forge Fixture",
            "--actor-email",
            "forge@example.invalid",
            "--signing-key",
            str(self.key),
            "--allowed-signers",
            str(self.allowed),
        )
        return json.loads((self.output / "replay-receipt.json").read_text(encoding="utf-8"))

    def test_replays_root_merge_duplicate_tree_and_raw_messages(self) -> None:
        root = git(self.source, "rev-parse", "HEAD").decode().strip()
        multiline = self.raw_commit(b"subject\n\nbody line\n", root)
        unterminated = self.raw_commit(b"unterminated", root)
        merge = self.raw_commit(b"merge\n", multiline, unterminated)
        git(self.source, "update-ref", "refs/heads/main", merge)
        refs_before = git(self.source, "for-each-ref", "--format=%(refname) %(objectname)")

        receipt = self.replay(merge)
        mapping = dict(
            line.split("\t")
            for line in (self.output / "replay-map.tsv").read_text(encoding="ascii").splitlines()
        )

        self.assertEqual(receipt["commit_count"], 4)
        self.assertEqual(receipt["root_count"], 1)
        self.assertEqual(receipt["merge_count"], 1)
        self.assertEqual(receipt["unterminated_message_count"], 1)
        self.assertEqual(set(mapping), {root, multiline, unterminated, merge})
        self.assertEqual(commit_message(self.source, multiline), commit_message(self.output, mapping[multiline]))
        self.assertEqual(commit_message(self.source, unterminated), b"unterminated")
        self.assertEqual(commit_message(self.output, mapping[unterminated]), b"unterminated")
        self.assertEqual(
            git(self.output, "show", "-s", "--format=%P", mapping[merge]).decode().split(),
            [mapping[multiline], mapping[unterminated]],
        )
        self.assertEqual(
            git(self.source, "show", "-s", "--format=%at %ai%n%ct %ci", merge),
            git(self.output, "show", "-s", "--format=%at %ai%n%ct %ci", mapping[merge]),
        )
        self.assertEqual(git(self.source, "rev-parse", f"{merge}^{{tree}}"), git(self.output, "rev-parse", f"{mapping[merge]}^{{tree}}"))
        self.assertEqual(git(self.source, "for-each-ref", "--format=%(refname) %(objectname)"), refs_before)
        self.assertFalse((self.output / "objects/info/alternates").exists())
        self.assertNotEqual(
            git(self.source, "rev-parse", "--path-format=absolute", "--git-common-dir"),
            git(self.output, "rev-parse", "--path-format=absolute", "--git-common-dir"),
        )
        for target in mapping.values():
            git(
                self.output,
                "-c",
                "gpg.format=ssh",
                "-c",
                "gpg.ssh.program=ssh-keygen",
                "-c",
                f"gpg.ssh.allowedSignersFile={self.allowed}",
                "verify-commit",
                target,
            )

    def test_refuses_an_existing_output(self) -> None:
        self.output.mkdir()
        result = subprocess.run(
            [
                "python3",
                str(REPLAY),
                "--source",
                str(self.source),
                "--revision",
                "HEAD",
                "--output",
                str(self.output),
                "--actor-name",
                "Forge Fixture",
                "--actor-email",
                "forge@example.invalid",
                "--signing-key",
                str(self.key),
                "--allowed-signers",
                str(self.allowed),
            ],
            capture_output=True,
            text=True,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("output already exists", result.stderr)

    def test_refuses_an_unsupported_source_header_without_retaining_output(self) -> None:
        root = git(self.source, "rev-parse", "HEAD").decode().strip()
        unsupported = self.raw_commit(
            b"encoded\n",
            root,
            extra_headers=(b"encoding ISO-8859-1",),
        )
        git(self.source, "update-ref", "refs/heads/main", unsupported)

        result = subprocess.run(
            [
                "python3",
                str(REPLAY),
                "--source",
                str(self.source),
                "--revision",
                unsupported,
                "--output",
                str(self.output),
                "--actor-name",
                "Forge Fixture",
                "--actor-email",
                "forge@example.invalid",
                "--signing-key",
                str(self.key),
                "--allowed-signers",
                str(self.allowed),
            ],
            capture_output=True,
            text=True,
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("unsupported commit header 'encoding'", result.stderr)
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
