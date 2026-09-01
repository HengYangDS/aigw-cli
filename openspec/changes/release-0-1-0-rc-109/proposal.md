## Why

The accepted product now reports credential availability without exposing
secret values or treating an unavailable credential backend as an absent
Account. These accepted bytes are newer than rc.108 and need one immutable
release identity before installation and end-to-end client verification.

## What Changes

- Publish the accepted credential-observation correction as AIGW
  `0.1.0-rc.109`.
- Advance the version authority and release chronicle from the accepted source.
- Publish one signed source and tag object with identical assets to both
  Forges.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The observable behavior is already owned by the archived
`credential-observation` Change.

## Impact

Only release identity, chronology, and this bounded release Change are
modified. No credential backend, client projection, transport behavior,
compatibility path, Proxy coupling, dependency, or Forge-specific product
logic is added.
