## Why

AIGW currently stores provider API Tokens through the selected portable backend
but stores provider-diagnostic credentials through an independent native
keyring path. That parallel authority breaks deterministic `env` and file-only
operation and makes one-provider setup behave differently across hosts.

## What Changes

- Make one selected credential backend own API Tokens and provider-diagnostic
  credentials in separate typed slots.
- Define collision-free, reversible environment variable names for Account IDs
  and optional diagnostic credential pairs.
- Keep the environment backend read-only and require a complete diagnostic
  credential pair before enabling precise provider diagnostics.
- Preserve token-free manifest import, one-Account connection, absent-client
  deferral, and later idempotent synchronization.
- **BREAKING**: Require lowercase Account IDs so their portable environment
  projection is deterministic.
- Delete the independent diagnostic-keyring authority; no compatibility read or
  dual write remains.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `secret-storage`: One selected backend owns every AIGW credential purpose,
  with isolated typed slots and deterministic environment projection.
- `progressive-team-onboarding`: Team setup remains usable with zero or one
  connected Account and explains the exact next action when activation is
  deferred.

## Impact

The change affects AIGW-owned credential storage, Account diagnostics,
configuration validation, setup composition, tests, and user documentation. It
does not manage a Proxy, require every Provider, install a client, inspect
client-private state, or change external endpoint lifecycle.
