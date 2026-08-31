## Why

Token-free team setup currently names an arbitrary Account variable while
claiming that any compatible Account will work. If the operator later supplies
a Token for a different Account, `aigw sync` neither reselects the Routes nor
activates a newly installed client, so the guidance promises a journey the
product does not perform.

## What Changes

- Present every compatible Account activation choice without making any
  provider mandatory or selecting an arbitrary example.
- Make `aigw sync` select Routes from the Accounts currently available through
  the configured credential backend before client discovery and projection.
- Preserve one semantic plan for dry-run and apply; dry-run remains secret-free
  and non-mutating.
- Reuse the existing Account, Route, secret, synchronization, and projection
  authorities; add no state, command, proxy coupling, or compatibility path.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-control-plane`: make deferred Account connection and later client
  projection directly consumable for any compatible reviewed Account.
- `progressive-team-onboarding`: make setup guidance enumerate real choices and
  make synchronization activate the choice that becomes available later.

## Impact

This changes setup guidance, synchronization planning, and their existing
acceptance coverage. It does not read or expose Token values, alter credential
ownership, manage external proxy lifecycle, or introduce a new dependency.
