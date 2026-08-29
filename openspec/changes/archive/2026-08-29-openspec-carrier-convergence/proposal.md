## Why

The repository still tracks historical ETHOS Commitment files even though
official OpenSpec artifacts now own change intent and ETHOS compiles the
selected intent transiently. Removing the duplicate carriers restores one
authority and eliminates stale lifecycle state.

## What Changes

- Make official OpenSpec artifacts the only tracked representation of change
  intent.
- Define the ETHOS Commitment as a transient three-field projection of the
  selected OpenSpec change.
- Remove tracked root and per-change Commitment files after validating the
  official OpenSpec history that remains.
- Keep immutable Git history, official OpenSpec archives, and ETHOS
  Attestations as their existing evidence surfaces.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `repository-organization`: Narrow the semantic documentation architecture to
  one tracked change-intent authority and one transient ETHOS projection.

## Impact

This changes repository governance artifacts only. Product behavior, release
assets, installed client projections, public commands, and dependencies are
unchanged.
