## ADDED Requirements

### Requirement: Configuration evolution preserves retained credential readers

AIGW SHALL write the canonical configuration only in a form that every
published credential reader of the same schema version accepts. A Route
capability outside the vocabulary published with that schema SHALL be stored
in a separate configuration component that credential readers do not read,
while the full program SHALL read both.

#### Scenario: A Route declares a capability that earlier readers do not know

- **WHEN** a Route declares a capability outside the published schema
  vocabulary and AIGW saves the configuration
- **THEN** the canonical configuration SHALL contain only published-vocabulary
  capabilities
- **AND** every published credential reader of that schema SHALL return the
  projected Token from it
- **AND** AIGW SHALL still report and project the declared capability.

#### Scenario: The capability component is damaged

- **WHEN** the separate capability component cannot be parsed or validated
- **THEN** a credential reader SHALL still return the projected Token
- **AND** other configuration reads SHALL fail without changing stored files.

#### Scenario: Recovery restores a capability with its configuration

- **WHEN** AIGW compensates a configuration write, restores a backup or reads
  a verified checkpoint
- **THEN** each restored configuration SHALL declare the stored capabilities
  that applied to it
- **AND** a verified checkpoint SHALL contain only published-vocabulary
  capabilities.

### Requirement: Stored capabilities follow their recorded Route

AIGW SHALL accept a capability outside the published schema vocabulary at its
earlier location in the canonical configuration and SHALL move it to the
separate component at its next configuration write. A stored capability SHALL
apply only while its Route keeps the Account, endpoint, upstream model and
interface it was recorded for.

#### Scenario: An earlier release wrote the capability into the canonical configuration

- **WHEN** the canonical configuration declares a capability outside the
  published schema vocabulary
- **THEN** AIGW SHALL accept the configuration with that capability declared
- **AND** its next configuration write SHALL remove the capability from the
  canonical configuration without changing what the Route declares.

#### Scenario: The recorded Route changes

- **WHEN** the Route of a stored capability no longer exists or no longer
  matches the Account, endpoint, upstream model or interface it was recorded
  for
- **THEN** the capability SHALL NOT apply
- **AND** the next configuration write SHALL drop it.

### Requirement: Credential configuration failures name the reader and stage

When a credential reader cannot read, parse or validate the configuration, it
SHALL return no Token and SHALL report its own AIGW version and the failed
stage. A read failure SHALL also name the operating-system operation and error.
The report SHALL NOT contain a path, configuration content or a Token. After a
parse or validation failure, its recommended action SHALL name synchronization
with the installed AIGW and a client reload.

#### Scenario: A reader cannot validate a newer configuration

- **WHEN** a credential reader fails to validate the configuration
- **THEN** its diagnostic SHALL name the reader's AIGW version and the validate
  stage
- **AND** standard output SHALL stay empty
- **AND** the recommended action SHALL name `aigw sync` and a client reload.

#### Scenario: The configuration cannot be opened

- **WHEN** reading the configuration fails with an operating-system error
- **THEN** the diagnostic SHALL name the read stage, the failed operation and
  the error
- **AND** it SHALL NOT contain the configuration path.
