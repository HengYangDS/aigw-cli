## Why

The refresh of 2026-11-13 held Mise at 2026.10.25 while 2026.11.7 is the newest
stable release. Moving it showed a gap: AIGW's GitLab jobs on the macOS ARM64
host run the host's shared Homebrew Mise, which another owner updates and no
job checks. Once the declaration moves, those jobs would silently run another
release than the one that every other Forge job runs, which the supply-chain
specification forbids.

## What Changes

- Declare Mise 2026.11.7: the GitLab Linux image by its index digest, which also
  sets the version that GitHub's mise-action installs, and the executable and
  shim digests of AIGW's runner-owned Mise on the Windows ARM64 host.
- Give AIGW its own runner-owned Mise on the macOS ARM64 host,
  `/opt/mise-aigw-cli/bin/mise`, and make each of the four GitLab jobs there
  (`native-darwin`, `native-darwin-review`, `accepted-ref-parity`,
  `release-version`) verify its digest and version and put it first on `PATH`
  before any locked tool runs.
- Keep the repository Mise floor at 2026.10.25: no repository test asserts a
  diagnostic of a newer release, and the lock that 2026.11.7 writes installs
  under 2026.10.25, as the 2026-11-13 dev pipelines showed.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: AIGW's GitLab jobs on a shared macOS host verify and run a
  Mise that only AIGW's declaration updates.

## Impact

`.config/ci/pipeline.cue` and its generated projections, the retained-step
records, and the projection tests change. The Windows and macOS runner hosts
need the verified 2026.11.7 executables before the proposal's first pipeline.
The released program does not change.
