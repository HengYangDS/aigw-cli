## Why

The user directs that every supply chain move to its latest stable release
(2026-11-02, evening). A Renovate `dryRun=full` candidate calculation with the
repository policy on GitHub `dev` `3049f834` admits two updates published that
day: the Renovate image 44.202.1 and glab 1.127.0. The npm lock also has newer
releases within its declared ranges. OpenSpec 1.16.0 stays held until its
three-day release age ends at 2026-11-02T23:41Z, and Claude Code 2.1.316 until
2026-11-03T00:43Z. Go 1.27.3, the direct Go modules, Codex 0.166.0, Hermes
v0.22.5, every Action, the Mise image and the other Mise tools are already
current.

## What Changes

- Advance the Renovate image to 44.202.1, pinned to its index digest, and glab
  to 1.127.0 for all six locked platforms.
- Refresh the npm lock within its declared ranges under the three-day release
  age.
- Take OpenSpec 1.16.0 and Claude Code 2.1.316, or a newer stable release that
  the policy then admits, once their release ages end.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: a release from a source that cannot replace a published
  artifact advances without the minimum release age, which covers registries
  that can still unpublish or replace a release.

## Impact

The Mise declaration and lock and the npm lock change; with the client pin,
the CUE authority, both Forge projections, the projection tests and the
Windows client supplier name follow. The released AIGW programs, user
configuration and credentials do not change, so no release follows.
