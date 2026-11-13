## 1. Changelog

- [x] 1.1 Fold the 51 `0.1.0-rc.*` sections into the `0.1.0` section by category, newest first; all 230 items
      remain, `go run ./tools/release validate-changelog` passes, and it refuses an added untagged rc section.
- [x] 1.2 Restate the `release-distribution` changelog requirement as the rule the validator enforces, with a
      scenario for a release that follows candidates; strict OpenSpec validation of the Change passes.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head.
- [ ] 2.2 The hosted graph passes at the proposal head.
