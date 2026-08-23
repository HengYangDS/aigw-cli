## Why

The accepted AIGW source contains four signed corrections newer than rc.91:
repository-derived release identity, lifecycle-scoped CI evidence, corrected
OpenSpec archive chronology, and hermetic Git policy fixtures. The published and
installed product must identify those exact accepted bytes rather than leaving
`main` ahead of the release version.

## What Changes

Publish the current accepted behavior as rc.92 by advancing the version source,
recording the complete user-relevant delta once, and validating the existing
release contract without changing provider, credential, client, or projection
semantics.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change owns release identity and chronology only; the accepted
product behavior is already specified.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The exact signed product object remains the sole source published to GitLab and
GitHub.
