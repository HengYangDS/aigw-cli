## Why

AIGW declares Mise 2026.11.7 while 2026.11.14 (2026-11-24) is the newest
stable release and older than the repository's 24-hour release age. Moving it
showed a cost in the current layout: AIGW's own Mise on GitLab's macOS host
lives at a path that names no release, so each move replaces the file that
every running job of the earlier declaration verifies, and needs a window
without AIGW jobs.

## What Changes

- Declare Mise 2026.11.14: the GitLab Linux image by its index digest, which
  also sets the version that GitHub's mise-action installs, and the digest of
  AIGW's runner-owned executable on the macOS ARM64 host.
- Name the release in the macOS runner's Mise directory,
  `/opt/mise-aigw-cli/<release>/bin`, derived from the declared version. The
  host then holds the new release beside the earlier one before the
  declaration moves, and removes the earlier one once no job runs its
  declaration, so no move needs a window without jobs.
- Take jdx/mise-action v5.3.0, pinned by its commit.
- Keep the repository Mise floor at 2026.10.25.
- Drop the Windows shim wording left in the scenario `A runner owns its Mise`
  and in the dependency maintenance guide; since Windows support ended, no
  runner-owned Mise has a shim executable.

No product behavior changes, and nothing breaks for users.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: a runner holds a tool's new release before any job consumes
  its declaration without changing the executable of a running job; AIGW's
  macOS Mise is held in a directory per release, and the earlier release is
  removed once no job consumes its declaration.

## Impact

`.config/ci/pipeline.cue` and its generated projections, the retained-step
record, a projection test, the dependency maintenance guide and the changelog
change. The CUE authority stays the one owner of the pins; the existing macOS
preflight verifies the new directory, and no installer step joins the
pipeline. The GitLab macOS host needs the verified 2026.11.14 executable in its
new directory before the proposal's first pipeline; that installation and the
later removal of the earlier directory belong to the owner of the CI hosts
(design.md, Migration Plan). The host's shared Homebrew Mise and the released
program are out of scope.
