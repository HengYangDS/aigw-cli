## Why

The previously accepted supply-chain graph has acquired a newer stable
repository-tool image. AIGW must refresh that direct pin promptly while keeping
the existing ecosystem owners, deterministic locks, and product behavior intact.

## What Changes

- Advance the AIGW-owned Renovate image to its current stable release and bind
  it to the immutable published digest.
- Re-run the existing dependency, source, native, and release checks against the
  resulting graph.
- Add no updater, inventory, wrapper, compatibility path, or product behavior.

## Capabilities

This maintenance Change has no specification delta. Existing product and
repository contracts already require stable, reproducible supply-chain inputs.

## Impact

Only the existing dependency declaration and generated lock state may change.
Installed AIGW, user configuration, credentials, client projections, and
runtime behavior remain outside this Change.
