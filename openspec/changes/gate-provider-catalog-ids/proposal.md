## Why

ETHOS beta.12 names each gate provider by a catalog ID, such as `provider.code-behavior`, and keeps the former
`module:callable` strings only as legacy names that resolve to those IDs. A gate declared by a legacy name still runs,
but every proof reports `gate_provider_legacy_name` for it with the ID to declare instead. AIGW's profile declares
three such names, so each proof of this repository carries three notes that AIGW owns, and a later ETHOS release that
drops the legacy table would leave those gates unresolved.

## What Changes

- The `go-behavior` and `node-behavior` gates declare `provider.code-behavior` instead of
  `ethos.adapters.gates.code_quality:behavior_report`.
- The `go-static-analysis` gate declares `provider.code-static` instead of
  `ethos.adapters.gates.code_quality:static_report`.
- The profile test identifies the behaviour gates by the new ID.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. The three gates resolve to the same ETHOS callables, so no behaviour of the product or its proof changes, and
this Change declares `skip_specs: true`.

## Impact

`.ethos/profile.toml` and `tools/ci/ethos_profile_test.go`.

- **Authority boundary:** ETHOS's provider catalog owns the IDs and what they resolve to; AIGW's profile only names
  them.
- **Non-goals:** the eight other gates whose proofs report the same note. ETHOS's own default gates declare them, and
  their names are ETHOS's to change.
