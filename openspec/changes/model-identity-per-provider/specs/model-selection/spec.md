## MODIFIED Requirements

### Requirement: An added model takes only the family with its ID

A model that `aigw use` or `aigw model add` adds without `--family` SHALL take
an existing family whose ID equals its model ID, ignoring case, and otherwise
SHALL be its own family. AIGW SHALL NOT derive a family from a prefix or any
other part of an ID, so a variant SHALL join a family only through `--family`
or a team file. A family that `--family` creates SHALL be labelled with its own
ID, not with the model's.

#### Scenario: A model takes the family with its ID

- **GIVEN** family `minimax-m3` exists
- **WHEN** `aigw use claude ucloud/MiniMax-M3` adds that model
- **THEN** its family SHALL be `minimax-m3`.

#### Scenario: A variant does not join by its prefix

- **GIVEN** family `gpt-6.1-sol` exists
- **WHEN** `aigw use codex dmxapi/gpt-6.1-sol-cdx` adds that model
- **THEN** the model SHALL be its own family
- **AND** `aigw model edit dmxapi/gpt-6.1-sol-cdx --family gpt-6.1-sol`, or a
  team file, SHALL link it to `gpt-6.1-sol`.

#### Scenario: A new family is labelled with its own ID

- **WHEN** `aigw model edit dmxapi/gpt-6.2-sol-cdx --family gpt-6.2-sol` creates
  family `gpt-6.2-sol`
- **THEN** the family's label SHALL be `gpt-6.2-sol`.
