## Why

The accepted product now provides the Codex native-provider authentication
command already projected by AIGW. Those source bytes are newer than rc.100 and
need one immutable release identity before users can install and verify the
correction.

## What Changes

Publish the accepted Codex credential correction as rc.101 by advancing the
version authority and recording the user-visible fix once. The release commit
does not add another credential authority, client policy, or compatibility
layer.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The behavior is already specified and verified by the archived
`codex-provider-credential` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible asset matrix are
published unchanged to GitLab and GitHub.
