## ADDED Requirements

### Requirement: A check names why Codex's files fall short

`aigw check` SHALL name a Codex model table that an earlier release marked stale, a table that another program
produced, a configured program that cannot produce a table and a missing configured program each as such. It SHALL say
that a Codex configuration file does not match the selected model only when the file's settings differ from what AIGW
writes for that model.

#### Scenario: Another program produced the table

- **GIVEN** the file at Codex's configured path changed after it produced the model table, and the new file can
  produce one
- **WHEN** `aigw check --depth local` runs
- **THEN** Codex's problem SHALL say that its configured executable changed after it produced the table, and not that
  its file does not match the selected model.

#### Scenario: The selected model changed after the last sync

- **GIVEN** the selected model's endpoint changed after the last sync
- **WHEN** `aigw check --depth local` runs
- **THEN** Codex's problem SHALL say that its configuration file does not match the selected model.
