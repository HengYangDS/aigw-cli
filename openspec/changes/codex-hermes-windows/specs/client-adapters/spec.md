## ADDED Requirements

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
