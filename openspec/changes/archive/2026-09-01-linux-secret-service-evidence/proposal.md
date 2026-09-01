## Why

The Linux source gate exposes one unproved behavior: failure to connect to the
native Secret Service. The contract must be made explicit and verified without
weakening coverage or adding a production-only test seam.

## What Changes

- Specify the fail-closed result when Linux Secret Service cannot be reached.
- Add one Linux-native regression for that existing production path.
- Keep the credential backend, dependency set, and coverage policy unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secret-storage`: clarify the observable connection-failure behavior of
  Linux native credential observation.

## Boundary

- **Authority:** `internal/secrets` remains the sole credential-backend owner.
- **Reuse:** use the existing `go-keyring` integration and Go test environment
  controls; introduce no alternate connector or test-only runtime abstraction.
- **Breaking changes:** none.
- **Non-goals:** no fallback, storage, prompt, UI, or provider behavior changes.

## Impact

One existing specification and one Linux-only test; no production API,
dependency, configuration, or runtime change.
