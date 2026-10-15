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

## 2. Verification and release

- [ ] 2.1 `mise run check` passes, the lane head is proved and landed locally.
- [ ] 2.2 With user authorization, publish and release through the existing
      release workflow; after the Homebrew upgrade, the operator's Hermes model
      choice survives `aigw sync`.
