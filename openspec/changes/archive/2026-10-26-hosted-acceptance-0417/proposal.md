## Why

The 0.4.17 proposal ([HengYangDS/aigw-cli#206](https://github.com/HengYangDS/aigw-cli/pull/206))
failed all three hosted native jobs:

- **Linux and macOS:** `TestGitLabUnixMiseMirrorFailureStaysOnSelectedPeer`
  expects the warning that Mise 2026.10.19 prints when it cannot confirm a
  mirrored release asset. The CUE model still pins Mise 2026.10.15 for every
  Forge job, and that release prints no such warning. Local proofs passed only
  because this host runs 2026.10.19.
- **Windows:** statement coverage was 94.92% against the 95% floor. Thirty-three
  statements added since 0.4.16 run on no platform: the retry fields of the
  human `aigw use` preview, the release Git metadata's peer selection, and retry
  configuration failures.

## What Changes

- The CUE model pins Mise 2026.10.19, the latest stable release, which also
  fixes several ways a mirror or repository could subvert verification. The
  Debian image digest and the runner-owned Windows ARM64 executable and shim
  digests come from the official release. The repository's Mise floor moves to
  the same release, so local verification and every Forge run share one Mise
  behavior. Both Forge projections are regenerated.
- Regression tests cover the uncovered 0.4.17 paths:
  - the human preview of declared retry budgets;
  - every branch of release peer selection;
  - retry declarations that conflict, a corrupt or unreadable retry component,
    and an invalid catalogue during rollback admission;
  - mutually exclusive retry intent in selection.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `product-control-plane`: native verification runs the declared Mise
  release locally and on every Forge, and a release that changes a tested
  native diagnostic advances the pin, the floor and the runner-owned Mise
  together.

## Impact

`.config/ci/pipeline.cue`, `.config/mise/config.toml`, both generated Forge
projections, the retained-step record of GitLab's Windows Mise preparation and
new tests change. Contributors need Mise 2026.10.19 or later.
GitLab's runner-owned Windows ARM64 Mise must be replaced before the next GitLab
job; GitLab is unreachable now, so that replacement precedes the GitLab
catch-up. No product behavior changes.
