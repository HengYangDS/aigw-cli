## 1. The reader's test

- [x] 1.1 Move the reader's cases from `tools/ci/release_age_test.go` to `tools/ci/test/release-age.test.mjs` and name
      the file in the Node test commands of `.ethos/profile.toml` and `tools/ci/quality.go`. Done when the Node test and
      the `tools/ci` tests pass and Node's coverage report lists `tools/ci/release-age.mjs`.

## 2. Verification

- [ ] 2.1 Pass `mise run check` on the lane head.
- [ ] 2.2 A fresh review of this Change by a reviewer who has not seen it. Done when its findings are answered in code
      or in this design.
- [ ] 2.3 Pass the proposal's hosted checks on both Forges.
