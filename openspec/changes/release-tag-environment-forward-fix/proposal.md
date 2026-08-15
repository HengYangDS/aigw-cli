## Why

Generic repository tests inherit release-tag variables from Forge jobs and then
mistake their fixture repositories for the selected product release. This made
the valid `v0.1.0-rc.80` release environment break unrelated native source
tests after local verification had passed.

## What Changes

- Isolate generic repository test fixtures from ambient Forge release-tag
  selection while preserving explicit provenance scenarios.
- Add a regression that executes the affected suite under a real tag-shaped
  environment.
- Publish the correction as the forward-only `0.1.0-rc.81` release; retain the
  failed rc.80 tags and runs as immutable evidence.

## Capabilities

No product capability changes. This is a test-harness and release-metadata
correction, so `.openspec.yaml` declares `skip_specs: true`.

## Impact

Only `tools/repository`, `VERSION`, and `CHANGELOG.md` change. AIGW's product
behavior, provider configuration, client projections, and Forge independence
remain unchanged.
