## Why

The reviewed team catalogue intentionally preserves Accounts that are not yet
connected on a particular workstation. `aigw doctor` currently treats every
catalogue Account as credential-required, so a valid one-provider installation
is reported unhealthy merely because optional Accounts remain available for
later connection.

## What Changes

- Define credential health from the Accounts selected by active client Routes.
- Keep unconnected catalogue Accounts available without reporting them as
  failures.
- Preserve failure diagnostics for a selected Account whose Token is missing.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: Clarify that diagnostics require credentials only
  for Accounts selected by active client Routes.

## Impact

The diagnostic collector, its focused tests, and the product-control-plane
contract change. Provider configuration, secret storage, client projection,
and external endpoint behavior remain unchanged.
