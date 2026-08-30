## Why

`aigw check` can currently validate only the default Route Token while reporting
another enabled client Route as ready. That false-green result can send a user
into a client that cannot authenticate.

## What Changes

- Require `aigw check` to confirm that every enabled client Route has its own
  Account Token before claiming health.
- Return the existing account-scoped Token recovery action for the exact client
  and Account that are not ready.
- Keep endpoint probing bounded to the selected default Route; `aigw test` and
  `aigw verify` retain their existing responsibilities.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: health reporting now fails closed when any enabled
  client Route lacks its selected Account Token.

## Impact

The change is limited to readiness evaluation and its acceptance regression. It
adds no command, state store, dependency, provider branch, or Proxy coupling.
