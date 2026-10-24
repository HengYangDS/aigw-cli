# Tasks

## 1. Names

- [x] 1.1 Make tests expect registry labels for clients and protocol labels in
      check, test, verify, sync, repair, migration, selection, route, account
      and credential output, and identifiers in `--for` help; they fail before
      the change.
- [x] 1.2 Add `configuration.ClientLabel`, use it and `ProtocolLabel` in every
      human renderer, list identifiers in `--for` help, and remove
      `invocation.Title`, the validator's `title` and
      `AdmittedClientLabelUsage`; the tests pass.
- [x] 1.3 Show the migration preview's Client Bindings and Recommendations as
      sections and count native targets in agreeing words; its test passes.

## 2. Surfaces

- [x] 2.1 Make adapter and repair tests expect each projection's surface
      identity; they fail before the change.
- [x] 2.2 Add `claude-settings` and `hermes-home-default` to the surface
      registry, carry the surface on `ProjectionPlan` from each adapter, and
      report it in repair previews; the tests pass.

## 3. Verification

- [x] 3.1 Record the change in `CHANGELOG.md` and the naming rule in
      `docs/experience/terminal-experience.md`.
- [ ] 3.2 `mise run check` and the full proof of the lane head pass.
