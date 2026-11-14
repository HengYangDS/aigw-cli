## Why

No job of either Forge declares how long it keeps its evidence. GitHub therefore kept every Actions artifact for
its default of 90 days, and GitLab kept job artifacts without an end. On 2026-11-14 the GitHub repository held
2,625 artifacts (8.3 GiB), the oldest of 2026-09-11, and the GitLab project 3.3 GiB. The owner had the artifacts
older than seven days deleted that day; without a declared lifetime they accumulate again.

## What Changes

- The CI model declares one lifetime of retained job evidence, seven days, and both projections carry it: every
  GitHub upload step as `retention-days`, every GitLab artifact declaration as `expire_in`.
- A projection test fails for an upload or an artifact declaration without that lifetime or with another one.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `ci-diagnostics`: retained job evidence has one declared lifetime.

## Impact

`.config/ci/pipeline.cue`, the two generated Forge files and one projection test. No product code. A release's
assets are Release assets, not job artifacts, and keep no lifetime. GitLab still keeps the newest artifacts of
each ref, which is that project's own setting.
