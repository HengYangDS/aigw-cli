#!/usr/bin/env python3
"""Replay one Git history into an isolated, Forge-specific identity graph."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


IDENTITY = re.compile(rb"^(?:author|committer) .* <[^>]*> (-?\d+) ([+-]\d{4})$")
SUPPORTED_HEADERS = {b"tree", b"parent", b"author", b"committer", b"gpgsig"}


@dataclass(frozen=True)
class SourceCommit:
    """Semantic fields that a Forge identity replay must preserve."""

    oid: str
    tree: str
    parents: tuple[str, ...]
    author_date: str
    committer_date: str
    message: bytes


def command(
    *args: str,
    cwd: Path | None = None,
    input_bytes: bytes | None = None,
    env: dict[str, str] | None = None,
) -> bytes:
    """Run one fail-closed subprocess and return its standard output."""

    result = subprocess.run(
        args,
        cwd=cwd,
        input=input_bytes,
        check=True,
        capture_output=True,
        env=env,
    )
    return result.stdout


def git(repository: Path, *args: str, **kwargs: object) -> bytes:
    """Run Git against one explicit repository."""

    return command("git", "-C", str(repository), *args, **kwargs)


def parse_source_commit(repository: Path, oid: str) -> SourceCommit:
    """Read semantic commit fields without normalizing message bytes."""

    raw = git(repository, "cat-file", "commit", oid)
    headers, separator, message = raw.partition(b"\n\n")
    if not separator:
        raise ValueError(f"commit object has no message separator: {oid}")
    tree = ""
    parents: list[str] = []
    dates: dict[bytes, str] = {}
    active_header = b""
    for line in headers.splitlines():
        if line.startswith(b" "):
            if active_header != b"gpgsig":
                raise ValueError(
                    f"unsupported continuation for commit header "
                    f"'{active_header.decode('ascii', errors='replace')}': {oid}"
                )
            continue
        active_header = line.partition(b" ")[0]
        if active_header not in SUPPORTED_HEADERS:
            raise ValueError(
                f"unsupported commit header "
                f"'{active_header.decode('ascii', errors='replace')}': {oid}"
            )
        if line.startswith(b"tree "):
            tree = line[5:].decode("ascii")
        elif line.startswith(b"parent "):
            parents.append(line[7:].decode("ascii"))
        elif line.startswith((b"author ", b"committer ")):
            match = IDENTITY.match(line)
            if match is None:
                raise ValueError(f"cannot parse commit identity timestamp: {oid}")
            field = line.split(b" ", 1)[0]
            dates[field] = f"@{match.group(1).decode()} {match.group(2).decode()}"
    if not tree or set(dates) != {b"author", b"committer"}:
        raise ValueError(f"commit object lacks required semantic fields: {oid}")
    return SourceCommit(
        oid=oid,
        tree=tree,
        parents=tuple(parents),
        author_date=dates[b"author"],
        committer_date=dates[b"committer"],
        message=message,
    )


def verify_signature(repository: Path, oid: str, allowed_signers: Path) -> None:
    """Require one target commit to verify under the explicit trust input."""

    git(
        repository,
        "-c",
        "gpg.format=ssh",
        "-c",
        "gpg.ssh.program=ssh-keygen",
        "-c",
        f"gpg.ssh.allowedSignersFile={allowed_signers}",
        "verify-commit",
        oid,
    )


def commit_identity(repository: Path, oid: str) -> tuple[str, str, str, str]:
    """Return stored author and committer names and emails."""

    fields = git(repository, "show", "-s", "--format=%an%x00%ae%x00%cn%x00%ce", oid)
    identity = fields.rstrip(b"\n").decode("utf-8").split("\0")
    if len(identity) != 4:
        raise ValueError(f"cannot parse target commit identity: {oid}")
    return identity[0], identity[1], identity[2], identity[3]


def verify_replay(
    source_repository: Path,
    target_repository: Path,
    source: SourceCommit,
    target_oid: str,
    mapping: dict[str, str],
    actor_name: str,
    actor_email: str,
    allowed_signers: Path,
) -> None:
    """Verify one mapped commit before any public ref can move."""

    target = parse_source_commit(target_repository, target_oid)
    expected_parents = tuple(mapping[parent] for parent in source.parents)
    if (
        target.tree != source.tree
        or target.parents != expected_parents
        or target.author_date != source.author_date
        or target.committer_date != source.committer_date
        or target.message != source.message
    ):
        raise ValueError(f"semantic replay mismatch: {source.oid} -> {target_oid}")
    if commit_identity(target_repository, target_oid) != (
        actor_name,
        actor_email,
        actor_name,
        actor_email,
    ):
        raise ValueError(f"target identity mismatch: {target_oid}")
    verify_signature(target_repository, target_oid, allowed_signers)


def replay(args: argparse.Namespace) -> dict[str, object]:
    """Construct and verify one complete identity graph in fresh object storage."""

    source = args.source.resolve()
    output = args.output.resolve()
    allowed_signers = args.allowed_signers.resolve()
    if output.exists():
        raise ValueError(f"output already exists: {output}")
    if not allowed_signers.is_file():
        raise ValueError(f"allowed signers file is missing: {allowed_signers}")
    git(source, "rev-parse", "--is-inside-work-tree")
    revision = git(source, "rev-parse", "--verify", f"{args.revision}^{{commit}}").decode().strip()
    source_oids = git(
        source, "rev-list", "--reverse", "--topo-order", revision
    ).decode().splitlines()
    if not source_oids:
        raise ValueError("source revision has no reachable commits")

    created = False
    try:
        command("git", "clone", "--quiet", "--bare", "--no-local", str(source), str(output))
        created = True
        alternates = output / "objects/info/alternates"
        if alternates.exists():
            raise ValueError("replay object database uses alternates")
        refs = git(output, "for-each-ref", "--format=%(refname)").decode().splitlines()
        for ref in refs:
            git(output, "update-ref", "-d", ref)

        mapping: dict[str, str] = {}
        roots = merges = unterminated = 0
        signing_program = args.signing_program or "ssh-keygen"
        for oid in source_oids:
            source_commit = parse_source_commit(source, oid)
            roots += not source_commit.parents
            merges += len(source_commit.parents) > 1
            unterminated += not source_commit.message.endswith(b"\n")
            commit_args = ["commit-tree", "-S", source_commit.tree]
            for parent in source_commit.parents:
                if parent not in mapping:
                    raise ValueError(f"source parent is not mapped: {parent}")
                commit_args.extend(("-p", mapping[parent]))
            environment = {
                **os.environ,
                "GIT_AUTHOR_NAME": args.actor_name,
                "GIT_AUTHOR_EMAIL": args.actor_email,
                "GIT_AUTHOR_DATE": source_commit.author_date,
                "GIT_COMMITTER_NAME": args.actor_name,
                "GIT_COMMITTER_EMAIL": args.actor_email,
                "GIT_COMMITTER_DATE": source_commit.committer_date,
                "GIT_CONFIG_GLOBAL": "/dev/null",
                "GIT_CONFIG_NOSYSTEM": "1",
            }
            target_oid = git(
                output,
                "-c",
                "gpg.format=ssh",
                "-c",
                f"gpg.ssh.program={signing_program}",
                "-c",
                f"user.signingkey={args.signing_key}",
                *commit_args,
                input_bytes=source_commit.message,
                env=environment,
            ).decode().strip()
            mapping[oid] = target_oid
            verify_replay(
                source,
                output,
                source_commit,
                target_oid,
                mapping,
                args.actor_name,
                args.actor_email,
                allowed_signers,
            )

        target_tip = mapping[revision]
        git(output, "update-ref", args.ref, target_tip, "0" * 40)
        (output / "replay-map.tsv").write_text(
            "".join(f"{source_oid}\t{mapping[source_oid]}\n" for source_oid in source_oids),
            encoding="ascii",
        )
        receipt: dict[str, object] = {
            "schema_version": 1,
            "source_tip": revision,
            "target_tip": target_tip,
            "target_ref": args.ref,
            "commit_count": len(source_oids),
            "root_count": roots,
            "merge_count": merges,
            "unterminated_message_count": unterminated,
            "semantic_fields": [
                "tree",
                "message_bytes",
                "author_timestamp",
                "committer_timestamp",
                "ordered_parents",
                "merge_topology",
            ],
        }
        (output / "replay-receipt.json").write_text(
            json.dumps(receipt, indent=2, sort_keys=True) + "\n", encoding="utf-8"
        )
        return receipt
    except BaseException:
        if created:
            shutil.rmtree(output, ignore_errors=True)
        raise


def parser() -> argparse.ArgumentParser:
    """Build the portable replay command grammar."""

    result = argparse.ArgumentParser()
    result.add_argument("--source", type=Path, required=True)
    result.add_argument("--revision", required=True)
    result.add_argument("--output", type=Path, required=True)
    result.add_argument("--ref", default="refs/heads/main")
    result.add_argument("--actor-name", required=True)
    result.add_argument("--actor-email", required=True)
    result.add_argument("--signing-key", required=True)
    result.add_argument("--signing-program")
    result.add_argument("--allowed-signers", type=Path, required=True)
    return result


def main() -> int:
    """Run the replay and render its verified receipt."""

    try:
        receipt = replay(parser().parse_args())
    except (OSError, subprocess.CalledProcessError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1
    print(json.dumps(receipt, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
