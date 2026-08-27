## Why

The accepted product now verifies the current Codex model catalog through a
public client boundary and uses one portable credential authority for setup,
diagnostics, and deferred client adoption. Those source bytes are newer than
rc.95 and require a unique immutable release identity.

## What Changes

Advance the version authority and release chronicle to rc.96 without changing
provider, credential, projection, client, or transport behavior in the release
commit itself.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The accepted behavior is already owned by the archived
`codex-model-catalog-verifier` and `unified-credential-authority` Changes.

## Impact

Only `VERSION`, `CHANGELOG.md`, and this bounded release Change are modified.
The resulting signed commit, annotated tag, and reproducible assets are
published unchanged to GitLab and GitHub.
