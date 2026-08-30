## Why

Profiles already declare exactly one client, but the connectivity and live
verification commands still accept a second client selector alongside a named
Profile. That redundant input creates an avoidable conflict state and leaves
the canonical specification describing invalid legacy Profiles that the
current schema cannot persist.

## What Changes

- Reject combined `--profile` and `--for` selection instead of reconciling two
  authorities.
- Keep `--for` as the explicit selector when no Profile is named and derive the
  client solely from a named Profile otherwise.
- Remove stale specification and diagnostic language that suggests a current
  Profile may omit its client or can be repaired by supplying another selector.
- Preserve the existing `Client -> Profile -> Account` route model and its
  one-time schema-v2 migration boundary.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `profile-client-selection`: Make `--profile` and `--for` mutually exclusive
  selection modes and remove impossible current-schema behavior.

## Impact

- Affected surfaces are the `test` and `verify` command parsers, the
  configuration query diagnostic, focused tests, and the canonical selection
  specification.
- No dependency, storage schema, provider, client Adapter, compatibility path,
  or Proxy coupling is introduced.
