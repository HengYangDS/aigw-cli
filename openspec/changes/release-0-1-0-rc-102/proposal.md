## Why

The accepted product now gives each admitted client one explicit Profile Route,
removes the contradictory global default, and migrates older configuration at
the read boundary. These accepted bytes are newer than rc.101 and need one
immutable release identity before users can install them.

## What Changes

Publish the accepted setup and routing simplification as rc.102 by advancing
the version authority and moving the existing Unreleased chronicle into one
dated release section. The release commit adds no route authority,
compatibility path, or client-specific exception.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The behavior is already specified and verified by the archived
`setup-user-journey` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible asset matrix are
published unchanged to GitLab and GitHub.
