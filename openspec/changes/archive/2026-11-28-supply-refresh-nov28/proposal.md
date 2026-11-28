## Why

The repository takes every declared input at its latest stable release that its 24-hour release age admits. A
Renovate dry run with the repository's own policy on 2026-11-28 at dev `4143fb7dc`, `mise latest` for every Mise tool
and the forges' release lists found eleven candidates. Three carry security fixes: gh 2.106.0 (seven advisories),
GoReleaser 2.20.3 (GHSA-qx28-5g8c-9fw6) and uv 0.13.10 (GHSA-vjvj-3m3f-fgmm). The repository owner asked, relayed on
2026-11-28, that every governed repository take OpenSpec's latest stable release, 2.2.0.

## What Changes

- uv 0.13.9 to 0.13.10, CUE 0.17.1 to 0.18.0, gh 2.105.0 to 2.106.0, glab 1.131.0 to 1.133.0 and GoReleaser 2.20.2 to
  2.20.3 in the Mise configuration and its lock.
- OpenSpec 2.1.0 to 2.2.0 and markdownlint 0.42.1 to 0.42.2 in the npm lock.
- `github.com/charmbracelet/x/ansi` v0.11.9 to v0.12.0, the one direct Go requirement behind.
- The native clients that CI installs for real-client acceptance: Codex 0.170.0 to 0.172.0, Claude Code 2.1.327 to
  2.1.332 and Hermes v0.23.2 to v0.23.4, by its release commit and installer digest.
- Held, with the reason in the design: the Renovate image and Hermes v0.23.5.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The refresh applies the accepted supply rules and changes none, so this Change declares `skip_specs: true`.

## Impact

`.config/mise/config.toml`, `.config/mise/mise.lock` and its npm sidecar, `package.json`, `package-lock.json`,
`go.mod`, `go.sum`, the CUE authority's `nativeClients` and the projections generated from it.

- **Authority boundary:** each pin stays with its one declared owner. The ETHOS runtime that this lane's proof runs
  under is the repository's runtime selector, which no tracked file holds.
- **Non-goals:** the held releases; any change of a supply rule.
