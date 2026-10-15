# Spec Delta

## ADDED Requirements

### Requirement: One command verifies each credential executable once

Within one AIGW command, credential reader preparation SHALL read the running
program and each versioned reader at most once to verify its content identity,
and SHALL install only bytes that this verification read. Every command SHALL
verify content again; no verified identity SHALL be reused by a later command
or keyed by file metadata. After client projection, AIGW SHALL refuse a
verified reader that disappeared or was replaced.

#### Scenario: A synchronization reuses its verified reader

- **WHEN** one command derives, verifies and projects a versioned credential
  reader
- **THEN** it SHALL read the program and the reader at most once each
- **AND** later steps of the same command SHALL use those verified identities.

#### Scenario: The program changes after verification

- **WHEN** the running program's bytes change after the command read them
- **THEN** a reader that the command installs SHALL contain the bytes it
  verified.

#### Scenario: A reader changes during projection

- **WHEN** a verified reader is removed or replaced before projection completes
- **THEN** AIGW SHALL refuse it and roll back the projection.

#### Scenario: A later command meets changed bytes

- **WHEN** a program or reader changed after an earlier command verified it,
  even with an unchanged size and timestamp
- **THEN** the later command SHALL verify the current bytes by content
- **AND** it SHALL refuse a reader whose bytes no longer match its receipt.
