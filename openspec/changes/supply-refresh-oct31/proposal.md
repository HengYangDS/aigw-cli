## Why

The user directs that every supply input stay on its latest stable release.
A Renovate `dryRun=full` candidate calculation with the repository policy on
GitHub `dev` (2026-10-31) admits two updates, the Mise image 2026.10.24 and the
Renovate image 44.198.3, and holds OpenSpec 1.16.0 until its three-day release
age ends on 2026-11-02. The newest native client releases past the release
age, Codex 0.165.0 and Claude Code 2.1.312, are ahead of their pins. Go
1.27.3, every direct Go module, Node 26.13.0, uv 0.13.6, the GitHub Actions
and Hermes v0.22.5 are already current.

## What Changes

- Advance the Mise image to 2026.10.24 with that release's Windows ARM64
  executable and shim digests, and the Mise floor, then regenerate both Forge
  projections and the retained-step digest of the GitLab Windows preparation.
- Advance the Renovate image to 44.198.3, pinned to its index digest.
- Pin Codex 0.165.0 and Claude Code 2.1.312 for the native client journeys.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: while the newest stable release of a declared dependency is
  inside the minimum release age, a refresh keeps the locked release, names the
  held release and when its age ends, and a later refresh takes it.

## Impact

The Mise declaration, the CUE authority, the retained-step record, both
generated Forge projections and two projection tests change. The GitLab
Windows runner's Mise and client supplier follow at the deferred GitLab
catch-up, before any aigw GitLab job consumes this declaration. The AIGW
program, its release artifacts, user configuration and credentials do not
change, so no release follows.
