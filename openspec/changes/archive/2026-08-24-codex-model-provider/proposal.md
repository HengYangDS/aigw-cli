## Why

AIGW currently projects every Codex Profile through the built-in `aigw`
provider. Codex already supports named native model providers with command-backed
authentication, so forcing every compatible endpoint through one provider
identity needlessly couples profile selection to a single projection shape.

## What Changes

- Add one optional `model_provider` field to Codex-scoped Profiles.
- Preserve `aigw` as the canonical default when the field is absent.
- Project an explicit provider through Codex's native provider and
  command-backed authentication tables.
- Persist the exact projected provider identity in AIGW's sidecar so update,
  transition, validation, and removal operate on one owned table.
- Omit AIGW's model catalogue and generic Codex login for explicit native
  providers.

## Capabilities

### Modified Capabilities

- `product-control-plane`: Profiles may select one Codex-native provider without
  moving Token ownership or transport lifecycle into AIGW.

## Impact

The change is confined to AIGW configuration, Codex projection, synchronization,
tests, and documentation. It does not introduce Account-level provider fallback,
legacy secret configuration, proxy lifecycle coupling, or Codex private-state
mutation.
