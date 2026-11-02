## Why

The user directs that every supply input stay on its latest stable release.
A Renovate `dryRun=full` candidate calculation with the repository policy on
GitHub `dev` (2026-11-02) admits four updates: the Mise image 2026.10.25, the
Renovate image 44.199.2, mise-action v5.2.5 and `go.yaml.in/yaml/v3` v3.0.6. It
holds OpenSpec 1.16.0 until its three-day release age ends at
2026-11-02T23:41Z. Codex 0.166.0 is past the release age. Claude Code 2.1.316
published the platform packages that the native hosts install between 00:39Z
and 00:43Z on 2026-10-31, so its age ends at 2026-11-03T00:43Z. Go 1.27.3, the
other direct Go modules, Node 26.13.0, uv 0.13.6, the other GitHub Actions and
Hermes v0.22.5 are already current.

## What Changes

- Advance the Mise image to 2026.10.25 with that release's Windows ARM64
  executable and shim digests, and the Mise floor, then regenerate both Forge
  projections and the retained-step digest of the GitLab Windows preparation.
- Advance the Renovate image to 44.199.2, pinned to its index digest, and
  mise-action to v5.2.5, pinned to its commit.
- Advance `go.yaml.in/yaml/v3` to v3.0.6.
- Pin Codex 0.166.0 for the native client journeys. Claude Code 2.1.316 and
  OpenSpec 1.16.0 keep their locked releases until their release ages end, and
  the next refresh takes them.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: when a release ships as several packages that were published
  at different times, its minimum release age runs from the latest of the
  packages that the supplying hosts install.

## Impact

The Mise declaration, the CUE authority, the retained-step record, both
generated Forge projections, the Go module files and two projection tests
change. The GitLab Windows runner's Mise and client supplier follow at the
deferred GitLab catch-up, before any aigw GitLab job consumes this
declaration. The released AIGW programs, user configuration and credentials do
not change, so no release follows. The next release builds with
`go.yaml.in/yaml/v3` v3.0.6, which quotes a literal `<<` string when it encodes
YAML.
