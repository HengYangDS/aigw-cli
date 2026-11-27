## Context

The proposal states why. Two committees prepared the design on 2026-11-25: the issue-1 committee (program identity)
and the end-state committee (providers, identity, adapters); their syntheses are kept under
`build/evidence/issue-1-committee-20261125/reviewer-synthesis.md` and
`build/evidence/end-state-design-20261125/reviewer-synthesis.md` of the main checkout. The owner chose, at 17:28 that
day, one schema 9 Change after the Windows removal and Change A; both have landed and shipped in 0.11.0. Change A
already made a missing program stop its client with a repair, and made Codex's record name the program that produced
its model table. This Change adds the declaration and removes the client-native path.

## Goals / Non-Goals

**Goals:** the operator's declaration of each client's program, read and never rewritten; one rule that resolves it;
schema 9 without the client-native keys; DR-0013.

**Non-Goals:** cloud providers and identity references; a built-in list of extension directories; reporting other
installations that may read Codex's home; reading `extensions.json` or running an editor's CLI; a generic adapter
engine; new clients.

## Decisions

### `aigw.toml` holds intent; observations go elsewhere

A client section declares `program` (an absolute path to the program) or `extensions` (the absolute path of an
editor's extensions directory), one of the two or neither. Both are syntax-checked when the file loads: absolute,
clean, no control character, not both. Whether the path exists is decided on each operation, never at load, so a
missing program stops only its client (Change A's behavior). AIGW writes `program` only when it adopts a program for a
client that declares neither, and never changes or removes a declaration afterwards; only `aigw client enable`,
`aigw client edit` and a configuration rollback do. What a run observed (the resolved path, the extension version)
goes to command output and to Codex's record, as today.

Alternative rejected: keeping `executable` with a narrower meaning (the issue committee's recommendation). The owner
chose the rename, so that no existing automatically written path silently becomes a declaration.

### One rule resolves a declaration

`adapter.ClientProgram` stays the only place that decides which program a client uses; it gains the extensions case and
returns the resolution (the path, how it was found, the extension version) or the stop. For `extensions`, the client's
adapter declares, as data, its extension ID and the program's path inside one installed version:

| Client      | Extension ID            | Program inside the extension                                               |
| ----------- | ----------------------- | -------------------------------------------------------------------------- |
| Codex       | `openai.chatgpt`        | `bin/<os>-<arch>/codex`, os `macos` or `linux`, arch `aarch64` or `x86_64` |
| Claude Code | `anthropic.claude-code` | `resources/native-binary/claude`                                           |

These layouts were read from the extensions' published packages by the issue committee (research-sources §1 and §5);
the Codex extension derives the directory as `bin/${os}-${arch}` itself. Claude Desktop and Hermes have no extension,
so `--extensions` is refused for them.

Resolution reads the directory's entries named `<extension ID>-<version>` or `<extension ID>-<version>-<platform>`,
where the platform, if present, is this host's (`darwin-arm64`, `linux-x64`, `linux-arm64`); drops every entry that
the directory's `.obsolete` file lists; takes the highest version by numeric dotted comparison; and requires its
program to be a runnable regular file. Cases, each with a test: no `.obsolete` (nothing is obsolete); an unreadable or
malformed `.obsolete` (the client stops, nothing is written); no entry, or none for this platform (the client stops
with "no installed extension"); the highest entry without its program (the client stops; a lower version is not taken,
since the editor runs the highest). VS Code marks a removed version in `.obsolete` and deletes its directory later
(research-sources §2), which is why the file is read; it is not an exact list of what is on disk, which is why only
its entries are dropped and nothing else is inferred from it.

### Adoption is visible and happens once

A client that declares neither adopts, in a converging command, the program that discovery finds: the PATH first,
then the client's fixed application path (Claude Desktop's application bundle). It writes that path as `program` and
says so in the command's output. Extension directories are not searched: the issue committee's member 4 and its
cross-examiner judged the built-in list of editor directories the part to cut, and a declared `--extensions` covers
every editor without one. Where a later need for automatic extension discovery would join: a list owned by
`discovery`, consulted after the fixed paths. Adding it is additive.

### The client flags

`--program <path>` replaces `--executable`; `--extensions <dir>` declares the alternative; the two exclude each other.
`aigw client enable` no longer requires either: a client enabled without one adopts at its first convergence, or stops
with the repair if none is found. A `--program` path that lies inside a directory named like the client's extension
(`<extensions dir>/<extension ID>-<version>...`) is refused, with the `--extensions <extensions dir>` command that
declares the same program without its version. Every repair that named `--executable` names `--program`.

### Projected follows enabled, targets and the record

Whether a client's files are projected followed from a non-empty `executable` in activation, the local check, Claude
Code's adapter and Codex's verification (the issue committee's D7). It follows from the enabled state, the targets and AIGW's
record, so a client whose program is declared but not yet resolved keeps its files' state, and a client stopped by a
missing program is reported as stopped, not as unprojected.

### The client-native path leaves

`authentication` and `model_provider` leave the client section and the recommendation; the client specification
loses `ClientNativeAuthentication` and `DefaultModelProvider`; Codex's table name is always AIGW's own provider, with
the credential command. No configuration seen so far uses them: the owner's `aigw.toml` and the shipped team file hold
neither key (read 2026-11-27). The accepted requirement "Client-scoped Codex native provider" is removed; nothing
replaces it until a cloud provider returns under DR-0013.

### Schema 9 refuses schema 8 with the edits

The store refuses a version 8 file before reading anything else, as it refuses any other version, but names the
edits that schema 9 needs: set `version = 9`; rename each client's `executable` to `program`; delete `authentication`
and `model_provider` wherever they appear. The text is fixed; nothing of the old file is parsed for it, so no schema 8
reader survives. After the edits, a leftover key is refused by the existing undefined-field refusal with its line.
Alternative rejected: an in-place converter, which DR-0012 already rejected as an importer and which would keep a
second schema in the program.

### The credential reader and its file name stay

The frozen view decodes each client's `enabled` and `authentication`. Every released reader from 0.7.0 to 0.11.0
treats an absent `authentication` as the provider-Token kind (read from each tag's
`internal/domain/credential_view.go` on 2026-11-27), and every client that remains uses a provider Token, so a
session that started before the upgrade keeps its Token. The view therefore keeps its file name; DR-0012 point 4 gains
the sentence that says why. This release's reader stops decoding `authentication`, so the view only shrinks: a client
that authenticated itself never called the reader, and a file that still holds the key is refused by the store for
every other command.

### Team files and rollback

A team file is the configuration without `[clients]`, so it moves to version 9 and loses the recommendation keys;
`aigw config export` writes version 9. A member on 0.11.0 cannot apply it, and `aigw self rollback` to a release
before this one refuses while `aigw.toml` is at version 9, as DR-0012 point 5 already provides. The upgrade note says
to keep a copy of `aigw.toml` before the edit.

## Risks / Trade-offs

- Every user edits `aigw.toml` once before any command runs. Mitigation: the refusal names the edits, and the
  CHANGELOG gives them with an example.
- A remote host whose only program is an editor extension needs one `--extensions` declaration. Mitigation: the
  missing-program repair names both flags.
- Several editor profiles with different versions: the highest wins (the committee's trade-off, unchanged).

## Migration Plan

The owner's `aigw.toml` declares four `executable` lines and nothing client-native (read 2026-11-27). After the
release is installed, AIGW's maintainer keeps a copy of the file and makes the three edits; `aigw check --depth
local` then proves the result. Rollback: restore the copy and `aigw self rollback`.

## Open Questions

None.
