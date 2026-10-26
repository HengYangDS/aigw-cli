## Why

On 2026-10-26 aigw-cli was rebound to ETHOS `v0.2.0-alpha.14` (runtime
`27b859aa`). Its in-place proof of `dev` `0438dd9b` failed in
`go-static-analysis`. ETHOS runs its own `go vet ./...`, and the `./...` pattern
also matches the ignored `build/` tree. Retained evidence there holds Go source
snapshots in mixed packages, so `go` reports "found packages … in build/evidence/…"
and fails, even though every source check passed. A fresh Work Lane has no such
files, which is why lane proofs did not show the defect.

A Renovate `dryRun=full` candidate calculation with the repository policy on
GitHub `dev` (`0438dd9b`, 2026-10-26) also admits three stable tool updates.

## What Changes

- Declare `ignore ./build` in `go.mod`, so the Go command's package patterns
  never match generated output or retained evidence.
- Advance gh to 2.103.2 and the Renovate image to 44.181.7 in Mise, and
  `jdx/mise-action` to v5.2.3 in the CUE authority. Then regenerate the lock
  and both Forge projections.
- Keep the Mise image at 2026.10.15. Its version must equal the Mise owned by
  the GitLab Windows runner, and the pending GitLab catch-up still runs
  pipelines that require 2026.10.15. The image moves together with the runner
  after GitLab is reachable again.
- Keep the Windows native client pins. Codex 0.164.0 passes the release age
  on 2026-10-26T04:52Z, so both pins move together in one later Change with a
  single Windows client journey.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `repository-organization`: generated output and retained evidence stay
  outside the Go source graph.

## Impact

`go.mod`, the Mise declaration and lock, the CUE authority and the generated
GitHub workflows change. The GitLab projection does not change, because neither
the Mise image nor the GitLab-only inputs move. The AIGW program, its release
artifacts, user configuration and credentials do not change.
