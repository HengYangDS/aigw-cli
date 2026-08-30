## Why

The accepted product now derives each health request from the selected
client protocol, so Claude Code and Codex are checked with their respective
request paths and authentication headers. These accepted bytes are newer than
rc.102 and require one immutable release identity before installation.

## What Changes

- Publish the accepted client-protocol health correction as rc.103.
- Advance the version authority and move the existing Unreleased correction
  into one dated release section.
- Add no new route authority, provider branch, compatibility path, or Proxy
  dependency.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The behavior is already specified and verified by the archived
`client-protocol-health` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible asset matrix are
published unchanged to GitLab and GitHub.
