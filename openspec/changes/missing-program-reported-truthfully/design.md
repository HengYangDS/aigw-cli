## Context

The proposal gives the defect and the owner's decision of 2026-11-25 17:28. The issue committee's synthesis
(`build/evidence/issue-1-committee-20261125/reviewer-synthesis.md` in the main checkout) names the code: a configured
program that is no longer runnable is replaced by the discovered one in `adapter.resolveExecutable`
(`internal/adapter/support.go`), and `client list` repeats that rule in its own `installedExecutable`
(`internal/cli/client/list.go`). Its experiment 1 (an exported copy, a fake `codex` in an extension directory, real
adapter calls) showed the stale loop described in the proposal.

## Goals / Non-Goals

**Goals:** one rule for which program a client uses; a missing configured program reported with a repair that works;
the Codex model table's producer recorded and compared without running a program; the real reason in `check`.

**Non-Goals:** the program-source declaration, extension directories and the schema change (the next Change, which
replaces the meaning of `executable`); running other Codex installations; reading editor manifests.

## Decisions

**D1. One rule.** A client's program is the configured one when one is configured, and the discovered one only when
none is configured; the adoption writes the path, as today. The configured program is never replaced by AIGW. The
rule has one owner in `internal/adapter`; `client list` and every adapter call it, and the duplicate in `list.go` is
deleted. Alternative rejected: keeping adoption on a dead path and reporting it, because the issue's user then gets
another program silently, which is the defect.

**D2. A missing program stops that client.** A client stops when its configured program is not runnable, or, the
coordinator's ruling of 2026-11-26 after the independent review, when it runs and cannot produce what AIGW owns for
the client: a Codex model table that it can neither regenerate nor prove to be its own. Every command that commits a projection
(`sync`, `use`, `config rollback`, `client edit`, `client enable`, `client disable`, `provider add`, `provider edit`,
`provider remove`, the `model` changes, `setup` and `self uninstall`, the coordinator's rulings of 2026-11-26) writes
none of a stopped client's files, its model table or its record, writes the rest of its change, and exits non-zero
through one reporting point, `invocation.Stopped`, with the cause and fix that `check` gives and a statement of what it
wrote; the earlier reading that only `sync` exits non-zero is reversed by those rulings. The fix is `aigw client edit <client> --executable <path>`
for a missing program and the same command, naming a Codex that prints `debug models --bundled`, for a table. A file
that the change drops is withdrawn all the same, because withdrawal needs no program. The decision is made once per
operation, in the adapter registry's plan (`PlanStopping`), and passed down: synchronization keeps the plan's stops in
its projection, applies only the rest, and reports them through a per-invocation record (`synchronization.Stops`, beside
`Follows`), not through the commit's error, so that a command that undoes a stored Token on a failed commit does not
undo it after a commit that succeeded. The record fails closed: a commit or a preview without one refuses before it plans,
with an error that names it, rather than drop a stop. `check` decides it in the registry's inspection, through the same function. A
preview of `sync` lists the stopped clients beside the others' files (`stopped_clients`, `sync` schema 5) and fails.
`client edit` and `client enable` refuse an `--executable` that is not an existing regular file with an execute bit,
by stat alone. Setup counts a client as installed through the same rule as every command. The texts name no path,
because a local check never shows one; `aigw client list` shows the path. A program that never produced a table and
cannot produce one keeps the earlier behavior: no table is written, and nothing stops; the coordinator accepted this
narrow form on the condition that it is no loop, which `TestACodexThatNeverProducedATableIsNoLoop` pins.

**D3. The table's producer.** Codex's record gains the producing program's path and the version that its package
manifest (`codex-package.json`) declares, beside the digest it already keeps. `check --depth local` compares the
configured program's path, the SHA-256 of its file and its manifest's version with the record, manifest with recorded
manifest, never with `--version` output (the coordinator's ruling of 2026-11-26); a program without a manifest is
compared by path and digest alone. The one declared manifest location is the one read on installations on 2026-11-25:
one level above `bin/codex` in Homebrew's cask and in ChatGPT.app's `codex-cli`; the editor extension's location was
read only in a downloaded package, not on an installation, so it is not declared and such a program has no manifest.
The manifest closes the case of a wrapper script that execs another file: its own digest stays across an upgrade, the
manifest's version does not. The check runs no program while the producer agrees with the record. When it differs,
the check runs the same read-only probes as a sync's plan, once per change of the program, so that it names the sync's own stop when the configured Codex cannot produce a
table, and `aigw sync` otherwise. A record of an earlier release names no producer and fails the check until the next
sync records one. The idea comes from magpie (`internal/agent/codex_async.go`, v0.1.4784), which reads the version this
way.

**D4. The real reason.** `check` names a table that an earlier release marked stale, a table whose producer differs, a
program that cannot produce a table and a missing program as such; "does not match the selected model" stays for a real
mismatch. Its requirement joins `client-adapters`, beside the missing program, because `projection-format` would pass
the repository's 500-line limit for a specification.

**D5. Dry run.** The dry-run documentation states that a dry run runs read-only probes of the client program in an
isolated temporary directory, as Codex's plan already does.

## Risks / Trade-offs

- A user who relied on the silent switch now gets a failed `sync` after an upgrade moves the program → the failure
  names the exact `client edit` command; the schema 9 Change removes the cause for extension directories.
- Hashing a large program in `check --depth local` costs time → measured by the committee: 131 to 154 ms for a
  285 to 291 MB program, warm cache; cold cache unmeasured. A check whose producer differs also runs the two read-only
  probes, 99 to 196 ms for one identification in the committee's experiment 2.
- A program that never produced a table and cannot produce one is not stopped (D2) → Codex runs without AIGW's table,
  as before this Change. It is no loop: `TestACodexThatNeverProducedATableIsNoLoop` runs sync, check, sync and check,
  and each sync succeeds without a table while each check passes and names no repair, so the narrow form stays (the
  coordinator's ruling of 2026-11-26).

## Migration Plan

None for `aigw.toml`: the schema and keys are unchanged. `sync --json` moves to schema 5 for `stopped_clients`.
