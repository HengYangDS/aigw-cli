## Why

Since 2026-11-13 the dependency policy declares a release age of zero hours, by the owner's rule for every governed
repository, and the owner asked that every supply chain move to the latest stable release. The candidate
calculation at `2741f81f` then reports updates of seven named dependencies and one lock refresh, and holds none. Two of them cannot simply be taken: Mise,
whose executable two CI runner hosts own, and a major release of the YAML library with a new import path. The
specification says how a runner-owned tool advances, but not what a refresh does while that has not happened.

## What Changes

- Advance the native clients to Codex 0.169.0 and Claude Code 2.1.325; the Windows client supplier name follows.
- Advance Prettier to 3.10.1 and the jsdom override to 31.0.1, and refresh the npm lock at the declared release
  age.
- Advance the Renovate image to 44.239.1, pinned to its index digest.
- Keep Mise at 2026.11.7 until one Change moves its pin together with a window on the macOS and Windows runner
  hosts, and state that rule in the `supply-chain` requirement.
- Keep `go.yaml.in/yaml` at v3 in this Change; v4 changes the import path of every consumer and gets its own Change.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: a refresh that cannot hold a runner's window keeps that runner-owned tool's pinned release and
  names the Change that moves it.

## Impact

The CI pipeline model with its two generated Forge projections, both projection tests that name the client pins,
the npm manifest and lock,
the Mise configuration and the `supply-chain` specification. No product code changes, and no released program
contains any of these inputs.
