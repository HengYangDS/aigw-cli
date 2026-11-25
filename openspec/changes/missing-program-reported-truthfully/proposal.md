## Why

GitLab issue 1 reports Codex in VS Code over Remote SSH: AIGW wrote an extension directory's versioned `codex` path
into `aigw.toml`, the extension upgraded, the old directory went away, and AIGW kept working against a program that
no longer exists. The issue committee (2026-11-25, material under `build/evidence/issue-1-committee-20261125/` in the
main checkout) proved, in an exported copy, a stale loop: after the program disappears, `aigw sync` keeps the dead
path, removes the model table, marks the record stale and exits 0; `aigw check` then says "does not match the selected
model" and repairs with `aigw sync`, which changes nothing.

On 2026-11-25 at 17:28 the owner chose the end-state path: this Change first, with no schema change, then one schema 9
Change that declares the program source. This Change makes the current behavior truthful; it does not add the
declaration.

## What Changes

- A program that was configured and is gone stays configured. AIGW no longer swaps in another discovered program for
  it, in `sync`, `client list` or anywhere else; one rule decides which program a client uses. A client without a
  configured program still adopts the one that discovery finds, as today.
- When the configured program is missing, `sync` leaves that client's files, model table and record unchanged and
  exits non-zero; `sync` and `check` give the same repair, `aigw client edit <client> --executable <path>`, for Codex,
  Claude Code, Claude Desktop and Hermes, never `aigw sync`.
- Codex's record of the model table names the program that produced it (its path beside the version and digest it
  already keeps). `check --depth local` compares that program with the one now configured, by path and file digest,
  without running it, and fails with repair `aigw sync` when they differ.
- `check` states the real reason for a stale or unproducible Codex table instead of "does not match the selected
  model".
- The dry-run documentation says that a dry run runs read-only probes of the client program in an isolated temporary
  directory.

Not breaking for configuration: `aigw.toml` keeps schema 8 and its keys. Output changes: the new reasons and repairs,
and `client list` reports a configured program that is missing instead of another one.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `client-adapters`: which program a client uses, and what happens when it is missing.
- `projection-format`: Codex's model table records its producing program, and check compares it.
- `secret-storage` (its check scenario of a missing executable) and `client-verification`: the repair for a missing
  program.

## Impact

`internal/adapter` (`support.go`, the four adapters' `Inspect` and converge paths, Codex's catalog and record),
`internal/cli/client/list.go`, `internal/cli/check`, their tests and golden files, the terminal-experience and client
documents, the accepted specifications above.

Out of scope: the program-source declaration and editor extension directories (the schema 9 Change); reading
editor manifests; running other installations of a client; the real Remote SSH acceptance.
