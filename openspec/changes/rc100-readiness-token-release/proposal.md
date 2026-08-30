## Why

The accepted product now prevents `aigw check` from reporting overall health
when any enabled admitted-client Route selects an Account whose Token is
missing. Those source bytes are newer than rc.99 and require one immutable
release identity before users can install and verify the correction.

## What Changes

Publish the accepted all-active-route readiness correction as rc.100 by
advancing the version authority and recording the user-visible fix once. The
release commit does not add another readiness policy, credential path, or
compatibility layer.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The behavior is already specified and verified by the archived
`readiness-route-token-closure` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible asset matrix are
published unchanged to GitLab and GitHub.
