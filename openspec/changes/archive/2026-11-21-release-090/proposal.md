## Why

`dev` holds one change that users wait for: Claude Code and Claude Desktop learn which Anthropic model a provider's
own model ID serves (`client-model-windows`), so a provider's channel of a Claude model is no longer a "Custom
model" in Claude Code and no longer held to 200,000 tokens in Claude Desktop. The owner asked on 2026-11-20 for a 1M
window in every client; this is the part of that for the two Claude clients, and it reaches no one until it is
released.

## What Changes

- `VERSION` names 0.9.0.
- `CHANGELOG.md` moves the pending entries under the heading `[0.9.0]` with the UTC day of the preparation.

Out of scope: any change of the product, its tests or its specifications; those are the archived Changes
`client-model-windows`, `credential-deadline-phase` and `single-run-scope`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes no behavior of the product, so it declares `skip_specs: true`.

## Impact

`VERSION` and `CHANGELOG.md`. The release tooling reads both: the first changelog heading must equal `VERSION`, and
the bundle, the tag and the cask take their version from it.
