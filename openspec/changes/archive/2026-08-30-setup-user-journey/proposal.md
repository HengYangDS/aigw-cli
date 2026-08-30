## Why

Team onboarding already supports optional Accounts, deferred client discovery,
and an explicit credential backend, but its machine interface does not expose
the setup result. It also preserves an older global-default Route beside
client-specific Routes. A concrete Profile declares one client and one model,
so that global default cannot coherently select both Claude and Codex. The two
route semantics let setup, `use`, readiness, and projection disagree about the
active service.

## What Changes

- Add a secret-free JSON result to manifest-based setup.
- Derive human and JSON output from one setup result so the two projections do
  not acquire parallel semantics.
- Replace global default plus client overrides with one explicit
  client-to-Profile Route map.
- Make `aigw use <profile>` select the Profile's declared client; remove the
  ambiguous default, inheritance, reset, and `--all` semantics.
- Make readiness inspect and probe the effective Route of every enabled client,
  deduplicating only identical authentication targets.
- Preserve current progressive behavior: zero or one available Account is
  sufficient, absent clients are deferred, and external endpoints remain
  outside AIGW lifecycle ownership.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `progressive-team-onboarding`: Make the already-required machine-readable
  onboarding result available from the public setup command and make its
  selected Routes explicit per client.
- `product-control-plane`: Establish one client Route model and remove the
  logically invalid global Profile fallback.

## Impact

- Configuration and manifest schemas advance once to remove the global default;
  the previous local schema is migrated deterministically from explicit client
  overrides and only then from a compatible client-scoped former default.
- Route, setup, readiness, synchronization, recovery, rename, and credential
  consumers use the same client Route map.
- No new dependency, credential store, provider class, client Adapter, proxy
  coupling, or lifecycle state is introduced.
