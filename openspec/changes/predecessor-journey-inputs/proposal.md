## Why

Release 0.11.0's candidate acceptance runs the native client journey with the published 0.10.0 as its baseline
(`accept-native --baseline-tag v0.10.0`). Two of its inputs no longer suit that baseline:

- The journey gives the baseline the shipped team file, which now declares context windows for nine models. 0.10.0
  refuses the field, as the release notes of 0.11.0 state ("schema 8 does not define the field
  providers.models.context_window"), so `setup --from` stops before the journey starts.
- The candidate judges every lifecycle step with `check <client> --depth client`, including the first one, whose
  client files 0.10.0 wrote. The release notes state that the first check after the upgrade fails for every Codex
  with a model table until one `aigw sync`, because 0.10.0's record names no executable; Hermes's file also changed
  form (its `models` list became a mapping keyed by model ID). The journey runs no sync before that judgement.

Both are readings of the code and the release notes; the candidate acceptance measures them.

## What Changes

- When the baseline is a published predecessor, the journeys that it starts (the per-client journey and the Codex
  connection-retries journey) apply the shipped team file without its models' context windows, the step that the
  release notes give a member on 0.10.0. The candidate's own journeys keep the windows, so their native checks still
  cover the declared windows.
- In the baseline step, the candidate runs its `sync`, with the existing checks that the configuration and both
  credential commands stay as they were, before it judges the client files that the baseline wrote. The other steps
  are unchanged.

Out of scope: any change of the product, its release notes or its specifications; the example of
`aigw client edit --via`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change changes test inputs of the release acceptance and no behavior of the product, so it declares
`skip_specs: true`.

## Impact

`tools/release/clients_test.go` and `tools/release/clients_retries_test.go`. Authority stays with the release
acceptance in `tools/release`; nothing is reused from or added to the product. No compatibility code: the removal of
the windows applies only to a published baseline that predates them and goes once the oldest admissible baseline
reads them.
