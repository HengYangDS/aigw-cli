## Why

Renovate 44.103.1 became the latest stable release after the preceding supply-chain Change closed. AIGW must advance its direct OCI input without reopening unrelated product work.

## What Changes

- Update the sole Renovate image declaration to 44.103.1 and its immutable multi-platform digest.
- Re-run existing dependency and source checks against the updated input.
- Add no new dependency owner, updater, wrapper, or product behavior.

## Capabilities

This maintenance Change has no specification delta. Existing repository-governance requirements already require current stable, reproducible supply-chain inputs.

## Impact

Only `mise.toml` changes. Product runtime behavior, user configuration, credentials, client projections, and release identity remain unchanged.
