## Why

The first 0.10.0 candidate failed its native acceptance on 2026-11-24 because the Hermes real-client journey in
`tools/release` still read `adopted_selections`, which Change `client-chosen-model` had replaced with
`followed_selections` (fixed by Change `release-journey-followed`). Those journeys compile only under the
`client_acceptance` build tag, so neither `mise run check` nor the proof runs them; a Change that renames a field of a
JSON document meets them only at a release candidate. CONTRIBUTING's "Native upgrade acceptance" does not say so.

## What Changes

- CONTRIBUTING's "Native upgrade acceptance" states that the real-client journeys read the product's JSON documents,
  run only in candidate acceptance and the hosted lifecycle runs, and must be searched when a Change renames or removes
  a field.

Out of scope: any change of the product, its tests or its specifications.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

None. This Change edits contributor guidance only, so it declares `skip_specs: true`.

## Impact

`CONTRIBUTING.md`. It is the 0.10.0 release head's last commit; the release bundle is rebuilt, notarized and accepted
again on it.
