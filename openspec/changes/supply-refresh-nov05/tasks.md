## 1. Refresh

- [x] 1.1 Advance the Renovate image to 44.215.0, pinned to its index digest,
      and goreleaser to 2.20.0, with `mise lock` entries for all six
      platforms; `tools/ci project --check` reports no drift. Done in
      `45bb1c3f`: each goreleaser checksum equals the release's published
      `checksums.txt`.
- [x] 1.2 Refresh the npm lock under the three-day release age, taking parse5
      8.1.0 and entities 9.0.0; `npm ci` succeeds and no deprecated package
      remains. Done in `45bb1c3f`.
- [x] 1.3 Enable lock-file maintenance in the Renovate policy under the same
      release age; `mise run dependencies:check` validates the policy, and a
      test fails if the setting is removed. Done in `45bb1c3f`:
      `TestRenovateProposesTransitiveLockRefreshes` fails without the setting
      and passes with it.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (`ced0c1bc`, statement coverage 95.22%).
- [x] 2.2 The hosted graph passes at the proposal head (PR 236 run
      41889324215).
