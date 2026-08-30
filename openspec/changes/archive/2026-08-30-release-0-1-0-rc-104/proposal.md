## Why

The accepted product now has one client-scoped Route authority, proves the
complete portable program lifecycle on every supported native platform, and
contains the minimal current dependency closure. These accepted bytes are newer
than rc.103 and need one immutable release identity before users can install
them.

## What Changes

- Publish the accepted route, lifecycle-evidence, and dependency-closure work
  as AIGW 0.1.0-rc.104.
- Advance the version authority and move the accepted changes from
  `[Unreleased]` into one dated release section.
- Build, verify, and publish one exact signed product object independently to
  GitLab and GitHub.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The observable behavior is already specified by the archived
`native-update-rollback-acceptance` and `route-authority-consistency` Changes;
the dependency refresh changes no product requirement.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
No compatibility path, alternate route authority, Proxy coupling, dependency
manager, or platform-specific product implementation is added.
