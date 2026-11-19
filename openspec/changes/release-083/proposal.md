## Why

Two archived Changes of this lane, `setup-partial-tokens` and `team-suheai`, are ready to ship: a team setup that no
missing, skipped or rejected Token can block, and Suhe.AI in the team file. A user is blocked by the first today, and
the owner asked for the release as 0.8.3.

## What Changes

- `VERSION` names 0.8.3.
- `CHANGELOG.md` moves the pending entries under the heading `[0.8.3]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the two Changes named above.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
