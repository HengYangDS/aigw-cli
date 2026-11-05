## 1. Removal

- [ ] 1.1 Remove `tools/forge project` and `publish-tag` and their tests, and
      point the operations guide and contributor guide at `ethos publish`.
- [ ] 1.2 Remove repository subcommands, options, fields, hooks and legacy
      readers without a consumer, with their tests.
- [ ] 1.3 Replace hand-written helpers with the Go standard library.
- [ ] 1.4 Keep one copy of each duplicated test fixture in the package that
      owns its subject.

## 2. Verification

- [ ] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head.
- [ ] 2.2 The hosted graph passes at the proposal head.
