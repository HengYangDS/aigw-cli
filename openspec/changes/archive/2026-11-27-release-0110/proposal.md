## Why

`dev` holds the ten Changes archived since 0.10.0: the declared context windows of Codex and Hermes
(`codex-hermes-windows`), the removal of Windows and Intel macOS (`drop-windows-and-intel-macos`), a missing client
program reported truthfully (`missing-program-reported-truthfully`), the team refresh of 2026-11-27
(`team-refresh-1127`), a faster proof (`proof-under-ten-minutes`), the selection matrix on every native platform
(`selection-matrix-native-platforms`), one release-age declaration and its reader's tests (`release-age-declaration`,
`release-age-reader-coverage`), Mise 2026.11.14 (`mise-2026-11-14`) and the acceptance inputs for a predecessor
release (`predecessor-journey-inputs`). None of it reaches a user until it is released.

## What Changes

- `VERSION` names 0.11.0.
- `CHANGELOG.md` moves the pending entries under the heading `[0.11.0]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the archived Changes named above.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
