## ADDED Requirements

### Requirement: An unchanged Codex program is probed once per process

Within one AIGW process, reading Codex's bundled model table SHALL start an unchanged Codex program at most once for
its version and once for its table, and every later read in the process SHALL receive the same identity, table and
failure. Concurrent reads of one unchanged program SHALL wait for its one probe.

#### Scenario: A sync plans and applies an unchanged Codex

- **GIVEN** a Codex program that prints its version and its model table
- **WHEN** `aigw sync` plans and applies Codex's files
- **THEN** the program SHALL have been started once with `--version` and once with `debug models --bundled`.

#### Scenario: A probe fails

- **GIVEN** a Codex program that reports its version and cannot print its model table
- **WHEN** the table is read twice in one process
- **THEN** both reads SHALL report the same failure with the same measured identity
- **AND** the program SHALL have been started once for its version and once for its table.

#### Scenario: Reads of one program overlap

- **WHEN** several reads of one unchanged program start while its probe runs
- **THEN** the program SHALL have been started once for its version and once for its table
- **AND** every read SHALL receive the same identity, table and failure.

### Requirement: A changed Codex program is probed again

A Codex program SHALL count as unchanged only while its configured path, the file that the path resolves to, that
file's bytes and the version that its package manifest declares stay the same; a change of any of them SHALL start the
program again at the next read. A read whose program file cannot be read SHALL start no program and SHALL keep nothing
for a later read.

#### Scenario: The program changes between two reads

- **GIVEN** a read of a Codex program's model table earlier in the process
- **WHEN** the program's bytes, the version that its package manifest declares, the file that its configured path
  resolves to or the configured path itself changes, and the table is read again
- **THEN** the program SHALL be started again.

#### Scenario: The program file cannot be read

- **GIVEN** a configured Codex path that names no file
- **WHEN** the table is read, the program is then written at that path, and the table is read again
- **THEN** the first read SHALL report that the program cannot be read and start nothing
- **AND** the second read SHALL start the program.
