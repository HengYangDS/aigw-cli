# Design

## Context

The Codex adapter's `Inspect` returns one `client.Check` per configured target,
with the ID `codex:target-<n>`, where `n` is the target's 1-based position in
the binding. Doctor copies each check into its own `Check` and renders the
label from the name alone, so the file never reaches the output.

Doctor also keeps private paths out of diagnostics, as the existing
`TestDoctorJSONDoesNotExposeMissingCodexTarget` requires, because operators
share its output.

## Goals / Non-Goals

**Goals:** an operator can tell which file each Codex target check is about,
without diagnostics exposing a private path.

**Non-Goals:** changing check names, the comparison itself, or the labels of
other checks; the wider command-surface redesign.

## Decisions

### Only a home-relative path is public

A file under the home directory is shown as `~/<path>`. That form carries no
user name, and it names the default Codex home exactly. A file elsewhere is not
named: its label keeps the position and says `(outside the home directory)`.
When the home directory is unknown, the label keeps the position alone, because
the file's location cannot be classified.

Two alternatives were rejected:

- Absolute paths in JSON and human output: they would expose private paths and
  reverse the existing privacy rule.
- Surface names such as "default Codex home": they would still need positions
  for several explicit homes.

### The adapter reports the file and doctor makes it public

`client.Check` gains `Target`, the absolute path of the file the adapter
inspected. Doctor converts it once, while collecting checks, into the public
`target` field, plus an unexported marker for a file outside the home directory.
JSON output and labels then derive from the same collected value. Re-deriving
the path from the binding in doctor was rejected: the adapter already knows
which file it read.

The home directory comes from the command's explicit environment (`HOME`, then
`USERPROFILE`), not from the ambient process.

## Risks / Trade-offs

- **A home-relative path names private directories under home.** The path
  reveals only what the operator placed under their own home and omits the user
  name. Paths outside home are never shown.
- **JSON consumers match on the name.** Names are unchanged, and `target` is
  added only for files under home.

## Migration Plan

None. The next release shows the file in doctor output.

## Open Questions

- None.
