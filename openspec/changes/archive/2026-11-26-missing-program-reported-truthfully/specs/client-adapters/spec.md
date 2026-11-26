## ADDED Requirements

### Requirement: A configured client program is never replaced

A client SHALL use its configured program whenever one is configured, and the program that discovery finds only when
none is configured, whose path AIGW then writes into the configuration. AIGW SHALL NOT replace a configured program
with another one in synchronization, the client list, a check or any other command, even when the configured program
is gone.

#### Scenario: The configured program is gone while discovery finds another

- **GIVEN** Codex is enabled with a configured program that is then removed, and discovery finds another `codex`
- **WHEN** `aigw sync` runs
- **THEN** the configuration SHALL keep the configured path
- **AND** AIGW SHALL NOT run the discovered program or write a file with it.

#### Scenario: No program is configured

- **WHEN** an enabled client has no configured program and discovery finds one
- **THEN** `aigw sync` SHALL write the discovered program's path into the configuration and synchronize the client with
  it.

#### Scenario: Setup takes the configured program first

- **WHEN** `aigw setup` decides which clients are installed while a client's configured program runs and discovery
  finds none
- **THEN** it SHALL count that client as installed.

#### Scenario: The client list shows the configured program

- **WHEN** `aigw client list` runs while a client's configured program is gone and discovery finds another
- **THEN** the client's row SHALL name the configured path and say that it is missing
- **AND** its JSON item SHALL carry that path as `executable` with `installed` false.

### Requirement: A missing program stops its client

When an enabled client's configured program is missing, or runs and cannot produce what AIGW writes for the client,
such as Codex's model table, every command that would write that client's files SHALL write none of them, its model
table or AIGW's record of them, apart from withdrawing a file that the change drops, SHALL write the rest of its
change, and SHALL exit non-zero with the cause and fix that the client's local check gives, never `aigw sync`.

#### Scenario: Codex's program is removed after a sync

- **GIVEN** `aigw sync` wrote Codex's configuration file, its model table and AIGW's record with a program that is
  then removed
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave the three files byte-identical and exit non-zero
- **AND** each SHALL name the missing program and the repair `aigw client edit codex --executable <path>`.

#### Scenario: The other clients continue

- **GIVEN** Claude Code's program is present and Codex's configured program is missing
- **WHEN** `aigw sync` runs
- **THEN** it SHALL synchronize Claude Code's settings
- **AND** it SHALL exit non-zero for Codex.

#### Scenario: A check finds a missing program

- **WHEN** `aigw check --depth local` finds that the configured program of Codex, Claude Code, Claude Desktop or Hermes
  is missing
- **THEN** that client's local depth SHALL fail with the missing program as its problem
- **AND** its repair SHALL be `aigw client edit <client> --executable <path>`.

#### Scenario: A runnable program ends the stop

- **WHEN** `aigw client edit codex --executable <path>` names a runnable program
- **THEN** AIGW SHALL synchronize Codex with that program.

#### Scenario: Codex cannot produce its model table

- **GIVEN** a sync wrote Codex's model table with a program whose file then changes into one that runs and prints no
  table
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave Codex's file, AIGW's record and the table byte-identical and exit non-zero
- **AND** each run and `aigw check --depth local` SHALL say that the configured executable cannot produce its model
  table, with the fix that names another Codex executable.

#### Scenario: A dropped file is withdrawn while the client is stopped

- **WHEN** `aigw client edit codex --target <file>` runs while Codex's configured program is missing
- **THEN** AIGW SHALL withdraw its settings, its record and its model table from the file that the edit drops
- **AND** it SHALL write nothing to the new file and exit non-zero with the stop.

#### Scenario: A selection meets a stopped client

- **WHEN** `aigw use codex <provider/model>` runs while Codex's configured program is missing
- **THEN** the selection SHALL be saved and Codex's files SHALL stay as they were
- **AND** the command SHALL keep its own report, its JSON fields among them, and then exit non-zero with the missing
  program, its fix and what became of Codex's files.

#### Scenario: A rollback meets a stopped client

- **WHEN** `aigw config rollback` runs while Codex's configured program is missing
- **THEN** `aigw.toml` SHALL be restored and Codex's files SHALL stay as they were
- **AND** the command SHALL exit non-zero, saying that `aigw.toml` was restored and that Codex's files were not
  written.

#### Scenario: A preview names the stopped clients

- **WHEN** `aigw sync --dry-run`, or the preview of another command that writes client files, runs while Codex's
  configured program is missing
- **THEN** it SHALL report its own plan of the other clients' files and then Codex's problem and fix, write nothing and
  exit non-zero.

#### Scenario: The repair refuses a program that cannot run

- **WHEN** `aigw client edit` or `aigw client enable` names an `--executable` that is not an existing regular file with
  an execute bit
- **THEN** it SHALL refuse before anything changes, without running that file.
