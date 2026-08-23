## Why

AIGW currently assumes that the native credential service is continuously
available. That makes otherwise valid local-first setup fail on headless Linux
and constrained macOS hosts even when an operator has supplied one usable
provider Token.

## What Changes

- Select one writable Account Token backend from an explicit host snapshot.
- Keep the native credential service as the default on Windows.
- On macOS and Linux, use an owner-only AIGW file store only when the native
  credential service is unavailable.
- Persist the automatic backend choice so later invocations do not alternate
  between stores.
- Keep `env` explicitly selected, read-only, and non-persistent.
- Fail closed for unsafe file ownership, links, types, or permissions.

## Capabilities

### New Capabilities

- `secret-storage`: Portable and deterministic storage of Account Tokens.

### Modified Capabilities

None.

## Impact

The change is confined to AIGW-owned platform paths, Account Token storage,
CLI composition, focused tests, and user documentation. It does not read or
modify client private state, manage an external proxy, or add a compatibility
storage path.
