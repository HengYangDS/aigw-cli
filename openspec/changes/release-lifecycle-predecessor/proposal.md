# Release Lifecycle Predecessor

## Why

The read-only GitHub Release workflow verifies an existing release on a
selected native runner, and with `native_lifecycle` it also runs the release
lifecycle against that runner's published archive. Since `eb2f4c44`
(2026-10-07), a lifecycle journey over prebuilt artifacts requires an explicit
published predecessor, but the workflow never supplies one. Every
`native_lifecycle` run therefore fails before its upgrade and rollback
journeys. On 2026-10-17 the Linux, macOS and Windows runs for `v0.4.3`
(38992231195, 38992234319 and 38992237341) passed tag, artifact and
signed-source verification, then failed with "prebuilt lifecycle requires an
explicit published predecessor".

## What Changes

- The Release workflow gains a `baseline_tag` input that names the published
  predecessor.
- With `native_lifecycle`, the workflow passes that tag, the peer and its
  repository to the release owner's native acceptance. Native acceptance
  downloads the predecessor's matrix from the same peer and verifies its
  signatures and source before it runs the upgrade and rollback journeys.
- Without a selected predecessor, the lifecycle journeys still fail before
  product execution.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ci-diagnostics`: gains the requirement "Published lifecycle verification
  selects its predecessor".

## Impact

The owner is the `githubRelease` projection in `.config/ci/pipeline.cue`, which
renders `.github/workflows/release.yml`. Native acceptance in `tools/release`
and its predecessor verification are unchanged. GitLab's `release-assets` job
runs no lifecycle journey and is unchanged. The operations guide documents the
new input.

Authority boundary: read-only verification of existing published releases; no
rebuild, retag, signing or publication. Breaking removals: none. Reuse:
`accept-native --peer --repository --baseline-tag`, which the manual Verify
workflow already uses for published candidates.

Non-goals: choosing a predecessor automatically; lifecycle verification on
GitLab; changing native acceptance or its journeys.
