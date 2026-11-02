## Why

GitLab's native jobs run the repository's tests on Linux, Windows and macOS
ARM64 hosts, and since the GitLab peer caught up with `3049f834` its native
jobs fail on tests that assume GitHub's hosts:

- The native client supply tests require an official client layout for the
  test host, which GitLab's Linux and Windows ARM64 hosts lack.
- The Mise configuration boundary test removes the Mise data and cache
  directories that the GitLab jobs select, so its probe installs Go again and
  is stopped at its ten-second bound.
- The selection preview test looks for the target's file name in rendered
  output that wraps a long path at a position that depends on the length of
  the temporary directory.
- On the loaded Windows ARM64 runner, a release test reads a helper process's
  readiness record while the host still holds it, and a PowerShell witness
  passes but exceeds its fifteen-second bound.

## What Changes

- On a host without a declared client layout, the host supply test verifies
  that the command refuses the host before any effect, and the process-level
  interruption test, which needs the supply's installers to start, skips.
- The Mise configuration boundary test keeps its caller's Mise data and cache
  directories, never installs a tool, and still isolates Mise's configuration
  and state.
- The selection preview test finds the complete target path wherever the
  output wraps, and names the check that failed.
- The helper processes' readiness records are read through the retrying
  reader that tolerates transient Windows sharing violations, and the bounds
  of the PowerShell witness and the Mise probes stop only a hang.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: tests keep the Mise tool directories that their caller
  selected and install no tool, and the native tests pass on every hosted
  platform.
- `ci-diagnostics`: assertions on rendered output do not depend on where it
  wraps.

## Impact

Tests change; no product code, release artifact, configuration or credential
changes, so no release follows. The GitLab pipelines of the published head
qualify the change on the GitLab hosts. Out of scope: GitLab runner capacity
and concurrency, the Debian package download time of GitLab Linux jobs, and
native client acceptance on Linux or Windows ARM64.
