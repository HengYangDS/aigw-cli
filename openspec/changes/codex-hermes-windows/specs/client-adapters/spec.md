## ADDED Requirements

### Requirement: Codex runs the selected model at its declared context window

In the model table that AIGW writes for Codex, the entry that describes the selected model SHALL carry the model's
declared context window as its `context_window` and as its `max_context_window`. Every other field of that entry
and every other entry SHALL stay as Codex's own table has them. For a model whose family Codex's table does not
hold, AIGW SHALL write no entry, and `aigw check codex` SHALL say that Codex's own fallback window applies.

#### Scenario: The table holds the model's family

- **GIVEN** the selected model is a provider's ID of a family that Codex's table holds
- **WHEN** AIGW projects Codex's files
- **THEN** the alias that AIGW adds for the model SHALL carry the declared window in both fields
- **AND** the alias's other fields SHALL be the family's, apart from its own ID and display name
- **AND** the family's own entry and every other entry SHALL be unchanged.

#### Scenario: The model's ID is an entry of the table

- **WHEN** the selected model's ID is itself an entry of Codex's table
- **THEN** that entry SHALL carry the declared window in both fields in the table that AIGW writes
- **AND** nothing else of the table SHALL change.

#### Scenario: The table does not hold the model's family

- **WHEN** the selected model's family is not in Codex's table
- **THEN** AIGW SHALL write the table without an entry for the model
- **AND** `aigw check codex` SHALL report, in a check of its own that passes, that Codex has no entry for the
  model's family and uses its own fallback window whatever the model declares.

#### Scenario: The declared window changes

- **WHEN** `aigw model edit` changes the declared window of the model that Codex selects
- **THEN** the command SHALL rewrite Codex's table with the new window
- **AND** `aigw sync --dry-run` afterwards SHALL plan no change.

### Requirement: Hermes runs each model at its declared context window

For every model of every provider entry that AIGW owns in Hermes's file, AIGW SHALL write the model's declared
context window as that model's `context_length`, under the entry's `models` as a mapping from the model's ID, in the
order in which AIGW lists the models. AIGW SHALL write no window on a provider entry or in Hermes's own `model`
block. Every reading of the models that an entry lists SHALL accept the mapping and the list of IDs that an earlier
release wrote.

#### Scenario: Every listed model carries its window

- **WHEN** AIGW projects Hermes's file
- **THEN** each provider entry that AIGW owns SHALL list each of its models as its ID with `context_length` set to
  the declared window, in AIGW's order
- **AND** no provider entry and no `model` block SHALL hold a `context_length` that AIGW wrote.

#### Scenario: A file that an earlier release wrote

- **GIVEN** Hermes's file lists an entry's models as a list of IDs, with AIGW's record of the file intact
- **WHEN** AIGW projects the file
- **THEN** it SHALL accept the file as its own and write the mapping.

#### Scenario: A model chosen in Hermes among mapped models

- **GIVEN** the file lists an entry's models as the mapping
- **WHEN** the operator selects another listed model in Hermes and `aigw sync` runs
- **THEN** Hermes's selection SHALL name that model, as it does for a list of IDs.

#### Scenario: The declared window changes

- **WHEN** the declared window of a listed model changes
- **THEN** `aigw sync --dry-run` SHALL plan a write of Hermes's file
- **AND** the command that changed the window SHALL write the new number for an enabled Hermes.
