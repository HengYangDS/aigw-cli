## Why

One proposal commit currently starts a branch pipeline and then an equivalent
review pipeline on each Forge. Accepted publication then starts separate `dev`
and `main` pipelines for the same product object. These executions repeat the
same quality graph without producing distinct evidence.

## What Changes

Route verification by lifecycle event instead of every visible ref. Developer
changes verify through review into `dev`; a maintainer's direct accepted
publication verifies through `main`; tags retain the release path; explicit
manual dispatch remains available. `dev` is an exact peer of accepted `main`,
not an additional CI evidence owner.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `product-quality`: one product commit produces one verification execution per
  Forge and lifecycle stage.

## Impact

Only the CUE CI authority, its generated projections, focused projection tests,
and the governing quality contract change. Product gates and runner coverage
remain unchanged.
