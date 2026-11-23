## Why

`dev` holds the selection work that the owner held back on 2026-11-22 until both of its root causes were fixed: a
model chosen inside Codex or Hermes is that client's selection for every command (`client-chosen-model`), a client's
file is one file however its path is spelled (`one-file-identity`), and whether a provider's Token is usable is one
judgement (`one-token-usability`). Both root-cause Changes landed on 2026-11-24. Two maintenance Changes ride with
them (`team-import-prunes-by-default`, `team-refresh-1123`). None of it reaches a user until it is released.

## What Changes

- `VERSION` names 0.10.0.
- `CHANGELOG.md` moves the pending entries under the heading `[0.10.0]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the archived Changes named above.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
