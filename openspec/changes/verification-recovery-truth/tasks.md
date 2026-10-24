# Tasks

## 1. Codex Verification Copy

- [x] 1.1 Make a copy test expect a relative user-owned `model_catalog_json` to
      name the same file by absolute path, and absolute or `~` references to
      stay unchanged; it fails before the change.
- [x] 1.2 Rebase relative user catalog references in
      `rebaseCopiedCodexCatalog`; the test passes.

## 2. Route-Keeping Retry

- [x] 2.1 Make verification and `aigw test` tests expect retry advice to keep
      `--route` for an explicit Route and to omit it otherwise; they fail before
      the change.
- [x] 2.2 Pass the explicit Route to the Codex and Claude verifiers and compose
      every such retry with `clientverification.Command`; the tests pass.

## 3. Catalogue Admission

- [x] 3.1 Make parser and `aigw models` tests expect a null or repeated model
      list and repeated identifier members to fail the observation, with empty
      and extension-field controls passing; they fail before the change.
- [x] 3.2 Refuse those responses in `ParseIDs`; the tests pass.

## 4. Copyable Help Commands

- [x] 4.1 Make presentation and help tests expect command labels to keep one
      line at widths 24 and 32 while descriptions wrap; they fail before the
      change.
- [x] 4.2 Add `presentation.Field.Command`, write such labels whole in stacked
      groups, and mark root help's command labels; the tests pass.

## 5. Backend-Aware Recovery

- [x] 5.1 Make check and balance tests on the environment backend expect the
      Token variable for HTTP 401 and both diagnostic variables for absent
      diagnostic credentials; they fail before the change.
- [x] 5.2 Apply the backend's recovery in `aigw check` and
      `aigw account balance`; the tests pass.

## 6. Export Keeps Bindings Local

- [x] 6.1 Make export tests expect the declared recommendations unchanged when
      bindings select an alternative or a manual-only Route; they fail before
      the change.
- [x] 6.2 Stop deriving recommendations from bindings in `configuration.Export`
      and say so in the team rollout guide; the tests pass.

## 7. Verification

- [x] 7.1 Record the changes in `CHANGELOG.md`.
- [ ] 7.2 `mise run check` passes on the lane head.
- [ ] 7.3 The full proof of the lane head passes.
