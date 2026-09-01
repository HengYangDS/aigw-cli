## Why

Read-only AIGW journeys currently call the credential value reader merely to
decide whether a Token exists. That can trigger native credential UI and turns
backend failures into a false "missing" result, so observation is neither
non-interactive nor truthful.

## What Changes

- Make credential availability an error-bearing, value-free observation.
- Make status, setup, sync, profile, route, adapter, manifest, doctor, and
  catalogue decisions use that observation before any credential value is
  needed.
- Keep value retrieval at authentication, validation, migration, and explicit
  credential operations only.
- Preserve one selected storage backend; do not add a cache, fallback store, or
  second credential authority.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secret-storage`: credential availability becomes non-interactive,
  value-free, and error-aware across supported operating systems.

## Impact

- Changes the internal credential-store contract and its callers.
- Adds platform-specific native metadata observation while retaining the
  current stable storage dependency for value operations.
- Does not change configuration, client projection, proxy lifecycle, session
  state, or public credential values.
