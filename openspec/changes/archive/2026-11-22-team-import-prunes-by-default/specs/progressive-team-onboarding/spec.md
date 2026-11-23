## ADDED Requirements

### Requirement: Importing a team file keeps only what something uses

`aigw setup --from <file>` SHALL remove, in the same transaction as the merge, every local-only model that no
client selects and no recommendation names, and then every family that neither a remaining model nor the file
names. Providers, their endpoints and Tokens, and every client's selection SHALL stay unchanged. With
`--keep-local`, the local-only models and families SHALL stay as they were.

#### Scenario: An older family beside its successor leaves on import

- **GIVEN** a configuration that holds `dmxapi/gpt-6.1-sol` and `mxapi/gpt-6.2-sol`, the team file holds only the
  latter, and no client selects the former
- **WHEN** `aigw setup --from <file>` runs
- **THEN** `dmxapi/gpt-6.1-sol` SHALL be gone when the command ends
- **AND** the report SHALL name it among the models that the import removed.

#### Scenario: A model that something uses stays

- **GIVEN** a local-only model that a client selects, or that a recommendation in the file or the configuration
  names
- **WHEN** `aigw setup --from <file>` runs
- **THEN** the model and its family SHALL stay.

#### Scenario: A local-only family whose every model is unused leaves

- **GIVEN** a family that only local-only models use, all of them unselected and unnamed by any recommendation,
  and that the file does not name
- **WHEN** `aigw setup --from <file>` runs
- **THEN** the family SHALL be gone when the command ends.

#### Scenario: Keep the local entries on request

- **WHEN** `aigw setup --from <file> --keep-local` runs
- **THEN** every local-only model and family SHALL stay as it was
- **AND** the report SHALL name what it kept.

#### Scenario: A preview names what the import would remove

- **WHEN** `aigw setup --from <file> --dry-run --json` runs
- **THEN** it SHALL report each model and family that the import would remove, and with `--keep-local` each one
  that it would keep
- **AND** it SHALL write nothing.

#### Scenario: A client's selection survives the removal

- **GIVEN** a configuration in which every client's selection is a model that the file names
- **WHEN** `aigw setup --from <file>` runs
- **THEN** every client's selection, every provider's endpoints and every Token SHALL be what it was.

#### Scenario: The removed flag is not accepted

- **WHEN** `aigw setup --prune` runs
- **THEN** the command SHALL refuse it as an unknown flag, like any flag that the command surface does not
  declare
- **AND** nothing SHALL change.
