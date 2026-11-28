## Why

`dev` holds six Changes archived since 0.12.0. Two reach users: `codex-lists-family-models`, which
makes Codex's model picker list only the selected provider's models (breaking), and `codex-probe-once`, which starts
each unchanged Codex program once per command. The others are the supply refresh of 2026-11-28, the ETHOS proof
route (`one-verdict-per-head`, `native-in-proof`) and the gate provider IDs. The owner reported on 2026-11-25 that
Codex's local model list was redundant; the fix on `dev` reaches the owner's machine only when it is released and
installed.

## What Changes

- `VERSION` names 0.13.0.
- `CHANGELOG.md` moves the pending entries under the heading `[0.13.0]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the archived Changes named above.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
