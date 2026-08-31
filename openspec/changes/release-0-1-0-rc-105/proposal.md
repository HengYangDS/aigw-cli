## Why

The accepted product now provides actionable progressive onboarding: one
compatible Account Token is sufficient, deferred credentials and clients have
one clear next action, and `aigw check` remains verification rather than hidden
activation. These accepted bytes are newer than rc.104 and need one immutable
release identity before users can install them.

## What Changes

- Publish the accepted progressive-onboarding correction as AIGW
  0.1.0-rc.105.
- Advance the version authority and move the accepted onboarding change from
  `[Unreleased]` into one dated release section.
- Build, verify, and publish one exact signed product object independently to
  GitLab and GitHub.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The observable behavior is already specified by the archived
`clarify-progressive-onboarding-next-action` Change.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
No compatibility path, alternate credential authority, Proxy coupling,
dependency, or platform-specific product implementation is added.
