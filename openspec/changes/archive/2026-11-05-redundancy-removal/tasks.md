## 1. Removal

- [x] 1.1 Remove `tools/forge project` and `publish-tag` and their tests, and
      point the operations guide and contributor guide at `ethos publish`.
- [x] 1.2 Remove repository subcommands, options, fields, hooks and legacy
      readers without a consumer, with their tests; doctor's inspection seam
      stays because its tests inject states no adapter reports.
- [x] 1.3 Replace hand-written helpers with the Go standard library, except
      where an empty result and a nil result admit different protocols.
- [x] 1.4 Remove the copied Codex verification runner and move the
      cross-command readiness test into acceptance; fixtures that a package's
      own tests need for its unexported types stay in that package.

## 2. Verification

- [x] 2.1 `mise run check` and `mise run dependencies:check` pass on the lane
      head (847156fd).
- [x] 2.2 The hosted graph passes at the proposal head (PR 239 at 5a543f8e).
