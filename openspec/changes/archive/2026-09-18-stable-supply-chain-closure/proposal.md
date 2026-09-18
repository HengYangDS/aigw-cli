## Why

AIGW has a closed product release, but repository-owned tool and dependency
versions continue to advance. The supply chain must be refreshed as one bounded
maintenance operation so local development, generated Forge projections, and
release construction consume the same current stable inputs.

## What Changes

- Audit every direct Go, Node, Mise, CI Action, container, quality, security,
  documentation, and release dependency against its authoritative stable
  release source.
- Advance compatible direct dependencies in their existing owners and let the
  native ecosystem resolvers select the transitive closure.
- Regenerate locks and CUE-owned Forge projections, then require a second
  resolution to be byte-clean.
- Remove superseded pins; do not add wrappers, compatibility aliases, another
  updater, or a parallel dependency inventory.

## Capabilities

This maintenance Change does not alter product behavior and therefore opts out
of specification deltas. The existing product-control-plane requirements own
the latest-stable and deterministic-resolution contract.

## Impact

The bounded repository scope is the existing dependency declarations and their
generated locks or CI projections. Installed AIGW, user configuration,
