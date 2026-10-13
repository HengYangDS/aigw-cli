# Spec Delta

## ADDED Requirements

### Requirement: One command verifies each credential executable once

Within one AIGW command, credential reader preparation SHALL read the running
program and each versioned reader at most once to verify its content identity,
and SHALL project only bytes that this verification read. It SHALL NOT accept a
program or reader whose bytes changed after they were verified, and file
metadata alone SHALL NOT establish that bytes are unchanged.

#### Scenario: A synchronization reuses its verified reader

- **WHEN** one command derives, verifies and projects a versioned credential
  reader
- **THEN** it SHALL read the program and the reader at most once each
- **AND** later steps of the same command SHALL use those verified identities.

#### Scenario: A reader changes after verification

- **WHEN** a reader's bytes change after verification, even with an unchanged
  size and timestamp
- **THEN** AIGW SHALL NOT project or record those bytes as the verified reader.
