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

**D2. A missing program stops that client.** When the configured program is not runnable, `sync` changes none of that
client's files, its model table or its record, reports the client as failed and exits non-zero; other clients
continue as they do for any per-client failure. `check` reports the same failure. The repair in both is
`aigw client edit <client> --executable <path>`, the existing command, for all four adapters. Each adapter's `Inspect`
gives that repair for a missing program instead of `aigw sync`. The rule of what is missing has one owner in
`internal/adapter`; synchronization leaves a stopped client out of every plan of every command, the adapter registry
refuses to plan one and reports it before the client's own inspection, so the adapters keep no check of their own, and
`aigw sync` reports it through the ordinary failure output, with no new JSON field. The texts name no path, because a
local check never shows one; `aigw client list` shows the path.

**D3. The table's producer.** Codex's record gains the producing program's path beside the version and digest it
already keeps. `check --depth local` compares, without running any program, the configured program's path, the
SHA-256 of its file and the version that Codex's package manifest declares (`codex-package.json`, read on 2026-11-25 in Homebrew's
cask one directory above `bin/codex`, beside ChatGPT.app's `codex-cli/bin/codex` wrapper one level up, and beside the
extension's `bin/<os>-<arch>/codex`; each layout's location is declared once), and fails with repair `aigw sync` when any
differs. The manifest closes the case of a wrapper script that execs another file: its own digest stays across an
upgrade, the manifest's version does not. A program without a readable manifest is compared by path and digest only,
and the documentation says so. The record keeps the version as `--version` prints it, `codex-cli <version>`, and the
manifest declares the bare version (both read on 2026-11-25 for 0.171.0-alpha.13.2 and 0.172.0), so the check compares
the manifest's version with the last word of the recorded one. A record of an earlier release names no producer and
fails the check until the next sync records one. The idea comes from magpie (`internal/agent/codex_async.go`, v0.1.4784), which reads
the version this way; on 2026-11-25 ChatGPT.app's manifest read 0.171.0-alpha.13.2, the version that running the
program printed in the committee's experiment 2.

**D4. The real reason.** `check` names a stale table, a table whose producer differs, and a missing program as such;
"does not match the selected model" stays for a real mismatch. Its requirement joins `client-adapters`, beside the
missing program, because `projection-format` would pass the repository's 500-line limit for a specification.

**D5. Dry run.** The dry-run documentation states that a dry run runs read-only probes of the client program in an
isolated temporary directory, as Codex's plan already does.

## Risks / Trade-offs

- A user who relied on the silent switch now gets a failed `sync` after an upgrade moves the program → the failure
  names the exact `client edit` command; the schema 9 Change removes the cause for extension directories.
- Hashing a large program in `check --depth local` costs time → measured by the committee: 131 to 154 ms for a
  285 to 291 MB program, warm cache; cold cache unmeasured.

## Migration Plan

None: the schema and keys are unchanged.
