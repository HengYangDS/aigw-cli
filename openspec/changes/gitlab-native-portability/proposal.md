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
- On the loaded Windows ARM64 runner, a release test reads a file that another
  process holds just after its publication, and a PowerShell witness passes
  but exceeds its fifteen-second bound.

## What Changes

- On a host without a declared client layout, the host supply test verifies
  that the command refuses the host before any effect, and the interruption
  test, which needs the supply's installers to start, skips.
- The Mise configuration boundary test keeps its caller's Mise data and cache
  directories and still isolates Mise's configuration and state.
- The selection preview test finds the complete target path wherever the
  output wraps, and names the check that failed.
- Release tests read files through the retrying reader that tolerates
  transient Windows sharing violations, and the PowerShell witness's bound
  stops only a hang.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-quality`: tests keep the Mise tool directories that their caller
  selected, and the native tests pass on every hosted platform.
- `ci-diagnostics`: assertions on rendered output do not depend on where it
  wraps.

## Impact

Tests change; no product code, release artifact, configuration or credential
changes, so no release follows. The GitLab pipelines of the published head
qualify the change on the GitLab hosts. Out of scope: GitLab runner capacity
and concurrency, the Debian package download time of GitLab Linux jobs, and
native client acceptance on Linux or Windows ARM64.
