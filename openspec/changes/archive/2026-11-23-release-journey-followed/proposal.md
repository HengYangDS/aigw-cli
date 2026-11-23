## Why

The 0.10.0 candidate's native acceptance against `v0.9.0` failed at 06:43 on 2026-11-24 in the Hermes journey:
"synchronization preview did not adopt the native selection: map[]". The journey still reads `adopted_selections`
from `aigw sync --dry-run --json`, which Change `client-chosen-model` replaced with `followed_selections` (its
changelog entry: `aigw sync --json` no longer writes `adopted_selections`). The product reports the choice; the
journey reads a field that no longer exists. It runs only in candidate acceptance and the hosted lifecycle runs, so the
check and the proof did not meet it.

## What Changes

- The Hermes journey in `tools/release` reads the choice from `followed_selections`: the entry of Hermes names the
  model chosen in Hermes's file.

Out of scope: any change of the product or its specifications.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change corrects a release test only, so it declares `skip_specs: true`.

## Impact

`tools/release/clients_hermes_test.go`. No other reader of `adopted_selections` exists in the repository.
