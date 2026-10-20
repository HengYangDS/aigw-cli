# Claude Desktop Native Edits

## Why

Claude Desktop 3.1623.0 re-saved AIGW's third-party profile on 2026-10-20 with
its own configuration editor. The editor kept every AIGW value but wrote its
canonical form: it omitted `coworkTabEnabled`, `isClaudeCodeForDesktopEnabled`
and `inferenceGatewayAuthScheme`, which equal Desktop's defaults, reordered the
model entries and added its own `desktopHome`, `builtinBrowserEnabled` and
`banner` settings. AIGW compares the profile's raw bytes with its ownership
record, so it reported the save as a user edit. `aigw sync` then refused every
client, and status reported Claude Desktop as invalid. A projection would also
have replaced the whole profile and dropped Desktop's own settings.

## What Changes

- AIGW reads the managed meaning of its Claude Desktop profile: the selected
  gateway, credential helper and arguments, models and tool search, with Chat,
  Cowork and Code enabled, model discovery off, a helper-script credential and
  bearer authentication. An omitted setting takes Desktop's default, and every
  other key belongs to Claude Desktop.
- A profile whose managed meaning still matches AIGW's record counts as
  unchanged, even after Desktop re-saves it. AIGW keeps the profile's bytes and
  only refreshes its ownership record.
- A projection that changes AIGW's values keeps Claude Desktop's own settings.
- A changed managed setting still refuses, and the refusal names the changed
  settings and how to restore them.
- The ownership record keeps its version and raw-byte digest, so an earlier
  AIGW release still recognizes a profile that this release recorded.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: gains the requirement "Claude Desktop keeps its own
  profile settings".

## Impact

`aigw sync` and status accept Desktop's own saves of the AIGW profile and keep
Desktop's settings. Credential readers, which never read the Desktop profile,
are unaffected.
