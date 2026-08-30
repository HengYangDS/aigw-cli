## Why

AIGW already projects `aigw credential codex` for Codex Profiles with an
explicit native provider identity, but the private credential command accepts
only Claude. The projection must resolve the selected Account Token through the
same credential authority it declares.

## What Changes

- Make the private credential command accept every admitted client rather than
  one client-specific branch.
- Resolve the selected client Route and Account before returning its Token.
- Keep unsupported clients, disabled adapters, unresolved Routes, and missing
  Tokens fail-closed without writing to standard output.
- Reuse the existing Account, Route, Adapter, and Token-store authorities; add
  no provider special case, compatibility layer, or second credential store.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: require an explicit Codex native-provider
  authentication projection to retrieve its selected Account Token through the
  installed AIGW executable.

## Impact

- `internal/cli/credential` becomes client-neutral across the admitted-client
  set.
- Existing Claude behavior remains unchanged.
- Codex native-provider Profiles become usable without exposing Tokens in
  configuration.
- No Proxy, provider-wire, client-history, dependency, or public command
  surface changes.
