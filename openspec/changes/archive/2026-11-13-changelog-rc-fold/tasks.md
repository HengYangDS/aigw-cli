## 1. Changelog

- [x] 1.1 Fold the 51 `0.1.0-rc.*` sections into the `0.1.0` section by category, newest first; all 230 items
      remain, `go run ./tools/release validate-changelog` passes, and it refuses an added untagged rc section.
- [x] 1.2 Restate the `release-distribution` changelog requirement as the rule the validator enforces, with a
      scenario for a release that follows candidates; strict OpenSpec validation of the Change passes.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane head (`fc9051a9`, statement coverage
      95.55%).
- [x] 2.2 The hosted graph passes at the proposal head (`fc9051a9`: GitLab MR !194 pipeline 15406, GitHub PR 260
      run 43413333073).
