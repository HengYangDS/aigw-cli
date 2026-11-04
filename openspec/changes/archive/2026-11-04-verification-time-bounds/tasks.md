## 1. Bounds

- [x] 1.1 Give the selected native fixture's child run a three-minute hang
      bound, with its context fifteen seconds longer; `tools/release` builds
      and vets on macOS, Linux and Windows. Done in `ad44f041`.
- [x] 1.2 Repeat the Windows interruption test with windows of 0.3, 1, 3 and 9
      seconds until an attempt exercises the owned process chain. Each attempt
      keeps its cleanup bound at 2.7 seconds after its window, and the test
      file vets for `windows/arm64` and `windows/amd64`. Done in `ad44f041`.
- [x] 1.3 Declare `package_timeout = "20m"` in the coverage policy, refuse a
      missing, unparsable or non-positive value, and pass it to the coverage
      run as `-timeout`; `go test ./tools/coverage` covers the argument and
      the refusals. Done in `ad44f041`: the three refusal cases fail without
      the validation and pass with it.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (`ad44f041`, statement coverage 95.22%).
- [x] 2.2 The hosted graph passes at the proposal head, including GitHub's
      Windows jobs and GitLab's `native-windows-review` on the Windows ARM64
      VM (PR 235 run 41826603423; MR !183 pipeline 14121).
