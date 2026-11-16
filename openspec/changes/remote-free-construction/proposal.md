## Why

"Local-first independent publication topology" requires, in its scenario "No Forge is configured", that the
build remains complete when the repository is used with zero remote peers. It does not. Measured on
2026-11-16 by another session in a clone of `9a3cf734` without a remote:
`TestNativeAttributionSummaryRetainsItsNonqualifyingScope` fails with "goreleaser diagnostics prevent
qualification", and it passes in the same clone once a remote exists. GoReleaser's snapshot build warns that
it could not get a remote URL, and the common runner rejects qualification on a native warning, as "Warnings
are owned failures" requires.

## What Changes

- Release construction builds the portable artifacts in a checkout without a Git remote, and GoReleaser
  emits no diagnostic there.
- A test builds in such a checkout and names the scenario that it verifies.
- The classification of native diagnostics does not change: a warning of a native tool still prevents
  qualification.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The accepted scenario already states the behavior, so this Change declares `skip_specs: true`.

## Impact

`tools/release/construction` and its tests. The released program does not change.

- **Authority boundary:** construction reads the checkout's Git configuration and gives one build tool its
  input; it writes no Git configuration and selects no publication peer.
- **Reuse:** the Git environment that construction already prepares for GoReleaser when a checkout has
  several peers and no `origin`.
- **Non-goals:** how a peer is selected when remotes exist; any message or filter for the tool's warning.
