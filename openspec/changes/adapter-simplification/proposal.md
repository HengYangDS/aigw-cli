## Why

The client adapters repeat code that one owner should hold. A fresh-context
review traced each copy to its callers:

- Each adapter writes the same Converge preamble.
- The JSON encoders, the receipt slices and the `.aigw-state.json` suffix are
  copied between adapters.
- Codex keeps a lattice of forwarders that only tests call.
- Surface metadata holds one value or none.

Every copy is code to keep correct more than once.

## What Changes

- One Converge preamble helper in `internal/adapter`; each adapter keeps only
  its own targets rule.
- One indented JSON encoder, one raw-message encoder, one receipt slice and one
  state-file suffix in `internal/adapter`.
- Codex's test-only forwarders go; tests call the functions production calls.
- Surface metadata that never varies or is never read goes; the persisted
  `projection_mode` sidecar field stays.

Non-goals: product behavior, persisted files, CLI output and configuration
formats do not change, and no shared test-support API is introduced.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: a step that every client adapter repeats has one
  owner.

## Impact

`internal/adapter` and `internal/platform` change. The released programs
behave the same, so no release follows from this Change alone.
