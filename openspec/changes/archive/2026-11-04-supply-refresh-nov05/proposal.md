## Why

The user directs that every supply chain move to its latest stable release.
On 2026-11-05 at 01:42 CST, the Renovate candidate calculation with the
repository policy on `96b13e47` admits two updates:

- the Renovate image 44.215.0;
- goreleaser 2.20.0, published 2026-11-04T12:48Z.

A lock refresh under the three-day npm release age, which the calculation does
not perform, also finds parse5 8.1.0 (published 2026-11-01T14:21Z). parse5
brings entities 9.0.0, a transitive major release.

The calculation holds Claude Code 2.1.318 until its release age ends at
2026-11-06T21:55Z, and the Mise image 2026.11.0 remains held.

## What Changes

- Advance the Renovate image to 44.215.0, pinned to its index digest.
- Advance goreleaser to 2.20.0 for all six locked platforms.
- Refresh the npm lock under the three-day release age: parse5 8.1.0 and
  entities 9.0.0.
- Keep Claude Code at 2.1.316 and Mise at 2026.10.25 for the reasons that
  the 2026-11-04 refresh recorded.
- Report transitive lock moves in the candidate calculation, so that a lock
  refresh no longer finds updates that the calculation missed.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `supply-chain`: the candidate calculation also reports what a lock refresh
  under the release age would move.

## Impact

The Mise declaration and lock, the npm lock and the candidate calculation's
configuration change. The released programs, user configuration and
credentials do not change, so no release follows.
