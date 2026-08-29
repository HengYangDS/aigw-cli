## Why

The accepted product now gives executable recovery guidance for the read-only
environment credential backend across setup, routing, diagnostics, and
verification. Those source bytes are newer than rc.98 and require one immutable
release identity before users can install and verify them.

## What Changes

Publish the accepted environment-credential recovery UX as rc.99 by advancing
the version authority and recording the user-visible correction once. The
release commit does not introduce another credential policy or compatibility
path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The behavior is already specified and verified by the archived
`env-backend-guidance` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible asset matrix are
published unchanged to GitLab and GitHub.
