## Why

Codex's independent acceptance of the team manifest on 2026-10-22
(product-readiness Attestation `f7a80e60`) passed manifest import, export,
idempotence and the declared Route order. It found one gap: `aigw account list`
and `aigw account list --json` print `aihubmix, dmxapi, ucloud` although the
manifest declares DMXAPI, UCloud, then AIHubMix through Account `order` ranks
10, 20 and 30, and the JSON output itself reports those ranks. The command sorts
Account identities lexically instead of using the presentation order that
`Config.CatalogAccountIDs` already provides to the catalogue and Route views.

## What Changes

- `aigw account list`, human and JSON, lists Accounts through
  `Config.CatalogAccountIDs`: declared order first, then identity.
- Serialization, validation, migration and manifest export keep their lexical
  identity order.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: the shipped team catalogue requirement gains a
  scenario for Account list order.

## Impact

`internal/cli/account` and its acceptance test change. Configuration files,
manifests, credentials and client projections do not.
