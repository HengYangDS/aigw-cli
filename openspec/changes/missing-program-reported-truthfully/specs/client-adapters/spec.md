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

#### Scenario: The client list shows the configured program

- **WHEN** `aigw client list` runs while a client's configured program is gone and discovery finds another
- **THEN** the client's row SHALL name the configured path and say that it is missing
- **AND** its JSON item SHALL carry that path as `executable` with `installed` false.

### Requirement: A missing program stops its client

When an enabled client's configured program is missing, synchronization SHALL change none of that client's
configuration files, its model table or AIGW's record of them, SHALL synchronize the other clients as it would
without that client, and SHALL exit non-zero. Synchronization and the client's local check SHALL give the same
repair, `aigw client edit <client> --executable <path>`, and never `aigw sync`.

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

### Requirement: A check names why Codex's files fall short

`aigw check` SHALL name a stale Codex model table, a table that another program produced and a missing configured
program each as such. It SHALL say that a Codex configuration file does not match the selected model only when the
file's settings differ from what AIGW writes for that model.

#### Scenario: AIGW withdrew a table that it could not produce again

- **GIVEN** the configured program could not produce a model table at the last sync, and AIGW withdrew the table
- **WHEN** `aigw check --depth local` runs
- **THEN** Codex's problem SHALL say that its model table is stale, and not that its file does not match the selected
  model.

#### Scenario: The selected model changed after the last sync

- **GIVEN** the selected model's endpoint changed after the last sync
- **WHEN** `aigw check --depth local` runs
- **THEN** Codex's problem SHALL say that its configuration file does not match the selected model.
