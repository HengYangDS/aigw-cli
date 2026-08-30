## Why

Team onboarding already supports optional Accounts, deferred client discovery,
and an explicit credential backend, but its machine interface does not expose
the setup result. `aigw setup --from ... --json` is rejected even though the
progressive-onboarding contract requires human and JSON results to describe the
same state. This leaves automation without a stable way to distinguish imported
catalogue, connected Accounts, configured clients, and the next safe action.

## What Changes

- Add a secret-free JSON result to manifest-based setup.
- Derive human and JSON output from one setup result so the two projections do
  not acquire parallel semantics.
- Preserve current progressive behavior: zero or one available Account is
  sufficient, absent clients are deferred, and external endpoints remain
  outside AIGW lifecycle ownership.
- Keep the existing human output and non-manifest setup behavior unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: Make the already-required machine-readable
  onboarding result available from the public setup command.

## Impact

- `internal/cli/onboarding`: setup result construction and its two projections.
- `internal/cli/acceptance`: CLI regression and secret-free result assertions.
- No new dependency, credential store, provider class, client Adapter, proxy
  coupling, or lifecycle state is introduced.
