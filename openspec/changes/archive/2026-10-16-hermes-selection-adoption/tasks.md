# Tasks

## 1. Adopt native selection in synchronization

- [x] 1.1 Add failing regressions: a sync after a native Hermes model change
      inside the catalogue fails today; an ambiguous match, a changed provider
      entry and an explicit `use --for hermes` keep their refusals or results.
- [x] 1.2 Observe the native choice in `internal/hermes/configuration`, map it to
      one Route in the Hermes adapter, adopt it only in `aigw sync`, and report
      `adopted_selections`; the regressions and every existing Hermes test pass.
- [x] 1.3 Refuse a blank line between change items of one CHANGELOG category in
      `validate-changelog`, and remove the one such gap in `CHANGELOG.md`.
- [x] 1.4 Prove selection-only drift from the recorded providers, so a choice
      adopted while its Token was missing converges once the Token returns;
      the public sync regression covers missing, restored and converged steps.
- [x] 1.5 Report an adapter's failed self-compensation as a failed rollback so
      that synchronization keeps a reader its surviving configuration may
      reference; the regression drives an adapter through `CommitProjection`.

## 2. Verification and release

- [x] 2.1 `mise run check` passes, the lane head is proved and landed locally;
      release head `3a2fffdc`.
- [x] 2.2 With user authorization, publish and release through the existing
      release workflow; after the Homebrew upgrade, the operator's Hermes model
      choice survives `aigw sync`; released as `v0.4.2` and installed on
      2026-10-16.
