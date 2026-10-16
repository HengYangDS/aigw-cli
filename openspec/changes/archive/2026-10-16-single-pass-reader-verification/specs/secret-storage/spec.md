# Spec Delta

## ADDED Requirements

### Requirement: One command verifies each credential executable once

Within one AIGW command, credential reader preparation SHALL read the running
program once to verify its content identity, and each versioned reader once
before client projection and once after it, with memory bounded independently
of the program's size. It SHALL install only program bytes whose digest equals
that identity. No identity SHALL be reused by a later command or keyed by file
metadata.

#### Scenario: A synchronization reuses its verified reader

- **WHEN** one command derives, verifies and projects an existing versioned
  credential reader
- **THEN** it SHALL read the program once and the reader twice
- **AND** later steps of the same command SHALL use those verified identities.

#### Scenario: A command installs the reader

- **WHEN** the versioned reader is absent
- **THEN** AIGW MAY read the program once more to copy it
- **AND** it SHALL refuse the copy unless its digest equals the verified one.

#### Scenario: The program changes after verification

- **WHEN** the running program's bytes change after the command verified them
  and the reader must be installed
- **THEN** AIGW SHALL refuse the installation and leave no reader state.

#### Scenario: A reader changes during projection

- **WHEN** a verified reader is removed, replaced or rewritten in place before
  projection completes, even with an unchanged size and timestamp
- **THEN** AIGW SHALL refuse it and roll back the projection.

#### Scenario: A later command meets changed bytes

- **WHEN** a program or reader changed after an earlier command verified it,
  even with an unchanged size and timestamp
- **THEN** the later command SHALL verify the current bytes by content
- **AND** it SHALL refuse a reader whose bytes no longer match its receipt.
