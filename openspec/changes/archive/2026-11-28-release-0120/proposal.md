## Why

`dev` holds one Change archived since 0.11.0, `configuration-holds-intent`: schema 9, the program declaration by
`program` or `extensions`, the removal of the client-native Codex path, and forwarding through a service root, which
openai-responses-proxy 8.0.0 needs. The owner ordered it on 2026-11-25 as the third step after the Windows removal and
Change A. None of it reaches a user until it is released, and the proxy's cutover to 8.0.0
waits for this release to be installed.

## What Changes

- `VERSION` names 0.12.0.
- `CHANGELOG.md` moves the pending entries under the heading `[0.12.0]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the archived Change named above.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
