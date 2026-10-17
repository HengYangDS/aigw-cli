# Tasks

## 1. Supply the published predecessor

- [x] 1.1 Make the Release workflow projection tests expect a `baseline_tag`
      input and a native lifecycle step that passes it with the peer and
      repository to `accept-native`; they fail on the current projection.
- [x] 1.2 Add the input and arguments to `githubRelease` in
      `.config/ci/pipeline.cue` and regenerate both peers' projections; the
      projection tests pass.
- [x] 1.3 Document `baseline_tag` in `docs/operations/forge-operations.md`.

## 2. Verification and release

- [ ] 2.1 `mise run check` passes, and the lane head is proved.
- [ ] 2.2 With the fix on GitHub `main`, the Release workflow passes on its
      Linux, macOS and Windows runners for `v0.4.3` with `baseline_tag`
      `v0.4.2` and `native_lifecycle`.
