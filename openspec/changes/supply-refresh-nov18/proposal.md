## Why

"Latest stable repository-owned supply chain" requires each declared input at its latest stable release, and the
repository owner directed on 2026-11-18 at 09:06 CST, in another session, that every governed repository take its
whole supply at the latest stable releases. A sweep on 2026-11-18 (11:44 to 11:52 CST, at the lane head
`631032b30`: `mise latest` for each Mise tool, `go list -m -u`, the npm registry and the forges' releases) found
eleven candidates. npm 12.2.3, published at 00:32Z, turned the findings of the npm 12.2.2 bundle into a block in
ETHOS's dependency audit.

## What Changes

- npm 12.2.2 to 12.2.3, uv 0.13.8 to 0.13.9, gh 2.104.0 to 2.105.0, glab 1.130.0 to 1.131.0 and Syft 1.57.1 to
  1.57.2 in the Mise configuration and its lock.
- The Renovate image 44.242.0 to 44.251.0, pinned to its index digest.
- OpenSpec 2.0.0 to 2.1.0 in the npm lock.
- `golang.org/x/tools` v0.52.0 to v0.53.0, the one direct Go requirement behind; `go mod tidy` moves what it
  requires.
- The native clients that CI installs for real-client acceptance: Codex 0.169.0 to 0.170.0, Claude Code 2.1.325
  to 2.1.327 and Hermes v0.23.1 to v0.23.2 (by its release commit and installer digest), with the Windows client
  supplier name that follows from the first two.
- Held, with its owner and date in the design: Mise 2026.11.9 for the hosted runners, which the CUE authority
  derives from the same release as the runner-owned executables.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The refresh applies the accepted supply rules and changes none, so this Change declares `skip_specs: true`.

## Impact

`.config/mise/config.toml`, `.config/mise/mise.lock` and its npm sidecar, `package.json`, `package-lock.json`,
`go.mod`, `go.sum`, the CUE authority's `nativeClients` and the projections generated from it.

- **Authority boundary:** each pin stays with its one declared owner.
- **Non-goals:** the held Mise release; any change of a supply rule.
