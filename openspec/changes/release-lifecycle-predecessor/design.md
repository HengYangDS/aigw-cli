# Design

## Context

`githubRelease` in `.config/ci/pipeline.cue` renders the read-only Release
workflow. It checks the selected tag's pipeline evidence, downloads that peer's
assets and verifies their signatures and source. With `native_lifecycle`, its
last step runs `accept-native --artifacts dist`. Native acceptance over
prebuilt artifacts runs the lifecycle journeys from the published predecessor
it is given; with none, `journey_test.go` and `rollback_admission_test.go`
fail with "prebuilt lifecycle requires an explicit published predecessor". The
manual Verify workflow already selects a predecessor with `--peer=github`,
`--repository` and `--baseline-tag`.

## Goals / Non-Goals

**Goals:** a published release's native lifecycle can run on every offered
runner from an explicitly selected published predecessor.

**Non-Goals:** inferring the predecessor from tags or Homebrew; changing native
acceptance, its trust checks or its journeys; GitLab lifecycle verification.

## Decisions

### The caller selects the predecessor

A `baseline_tag` input names the predecessor, like the Verify workflow's input
of the same name. Choosing it automatically would have to decide between
maintenance and mainline tags; an explicit tag keeps that decision with the
maintainer, and native acceptance validates its form.

### Native acceptance acquires and verifies it

The step passes `--peer=github`, the repository and the tag to
`accept-native`, which downloads the predecessor's matrix and verifies its
signatures and source with the same trust inputs as the selected release. The
workflow adds no download or verification of its own.

## Risks / Trade-offs

- [A maintainer omits `baseline_tag`] → the lifecycle journeys fail before
  product execution with the existing explicit message; artifact and source
  verification still report their own result.
- [A maintainer selects an unintended predecessor] → the run qualifies upgrade
  and rollback from that tag only, and its dispatch inputs record the tag.

## Verification

On `c14fe8d1`, the changed `TestPublishedArtifactVerificationUsesExactTagAndPublicTrust`
and `TestPublishedNativeLifecycleUsesSelectedIsolatedRunner` failed as
expected: the lifecycle step ran `accept-native --artifacts dist` alone, and
the workflow had no `baseline_tag` input. With the projection change, only
`.github/workflows/release.yml` changed among the peer projections, and the
`tools/ci/projection` package passes.

With the fix on GitHub `main` (`9292b719`), the Release workflow ran for
`v0.4.3` with `baseline_tag=v0.4.2` and `native_lifecycle=true` on
`ubuntu-24.04`, `macos-26` and `windows-2025`. On each runner the published
native lifecycle step executed after artifact and source verification and
passed, including the published-predecessor journeys.
