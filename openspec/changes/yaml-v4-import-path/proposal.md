## Why

`go.yaml.in/yaml/v4` v4.0.0 (2026-11-04) is the newest stable release of the YAML library that AIGW uses to
read and write Hermes's configuration file; v3 receives security fixes only. In Go a new major is another
module with its own import path, so the supply rule's newest release is a migration, which the refresh of
2026-11-16 held for a Change of its own.

Two things of v4.0.0, read in its source, would change what an operator sees if only the import path moved.
Its encoder folds a line past column 80, where v3's never folds, and AIGW writes Hermes's file through that
encoder. And its node type gains a field, while AIGW stores the model selection that it replaced as that very
type in its JSON ownership record, which released programs read with unknown fields refused. No test of this
repository compares a written Hermes file or record byte for byte, so neither change would fail a test.
Once those bytes were pinned, v4 showed two more differences, and both keep the operator's text: it writes no
blank line after a folded block scalar, and it keeps the value of a folded scalar that keeps its final line
breaks (`>+`), to which v3 adds a line break. A duplicate key's refusal also reads differently.

## What Changes

- Tests pin the bytes that AIGW writes today into Hermes's configuration and into its ownership record, under
  v3, before anything else moves.
- The ownership record's form becomes AIGW's own: a type of this repository with exactly the fields that
  today's records hold, so its bytes do not follow a library's struct.
- The repository imports `go.yaml.in/yaml/v4` in place of v3, and AIGW's writer states its line width (no
  folding) and its indentation. The pinned bytes stay as they are, except the two differences above and the
  refusal's wording, which the design takes from v4.
- `projection-format` states what AIGW keeps of a Hermes configuration outside its own part.

No command, option or exit status changes. A Hermes file that AIGW writes changes only where v4 keeps the
operator's text that v3 changed, and one refusal's wording changes.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `projection-format`: a Hermes configuration keeps the keys, values, comments and lines that AIGW does not
  own, and AIGW's ownership record keeps one form across library versions.

## Impact

- `internal/adapter/hermes/` (the writer, the ownership record) and its tests; `tools/ci` and the other
  readers of YAML (import path only); `go.mod` and `go.sum`.
- Release 0.8.2, a patch: `dev` holds nothing unreleased since 0.8.1 (`d5a2f98f`).

Out of scope: what Hermes itself writes; YAML files of other clients (none today); a change of the record's
fields.
