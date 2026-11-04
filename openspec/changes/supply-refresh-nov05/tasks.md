## 1. Refresh

- [ ] 1.1 Advance the Renovate image to 44.215.0, pinned to its index digest,
      and goreleaser to 2.20.0, with `mise lock` entries for all six
      platforms; `tools/ci project --check` reports no drift.
- [ ] 1.2 Refresh the npm lock under the three-day release age, taking parse5
      8.1.0 and entities 9.0.0; `npm ci` succeeds and no deprecated package
      remains.
- [ ] 1.3 Enable lock-file maintenance in the Renovate policy under the same
      release age; `mise run dependencies:check` validates the policy, and a
      test fails if the setting is removed.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 2.2 The hosted graph passes at the proposal head.
