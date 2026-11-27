## MODIFIED Requirements

### Requirement: The client list shows each client's state

`aigw client list` SHALL list every client that AIGW supports, each with the program that it uses, as declared,
resolved from its declared extensions directory or found by discovery, or that it is not installed; the extensions
directory that it declares; whether it is enabled; and its selected `provider/model` or that it has none. Its JSON item
SHALL carry the program as `program` and a declared directory as `extensions`.

#### Scenario: List the clients

- **GIVEN** Codex is installed, enabled and selects `dmxapi/gpt-6.1-sol`, and
  Hermes is not installed
- **WHEN** `aigw client list` runs
- **THEN** Codex's row SHALL show its program path, that it is enabled and
  `dmxapi/gpt-6.1-sol`
- **AND** Hermes's row SHALL show that it is not installed, and Claude Code and
  Claude Desktop SHALL have rows too.

#### Scenario: A client declares an extensions directory

- **GIVEN** Claude Code declares `extensions = "<dir>"`, whose highest version of Claude Code's extension holds its
  program
- **WHEN** `aigw client list --json` runs
- **THEN** Claude Code's item SHALL carry `<dir>` as `extensions` and the resolved program's path as `program`, and no
  `executable`
- **AND** its human row SHALL name the declared directory beside that program
- **AND** when no installed version holds its program, the item SHALL carry `extensions` with `installed` false and no
  `program`.

## ADDED Requirements

### Requirement: Every client selects a compatible model

Commands and a team file's new providers SHALL declare only model protocols
that the provider has an endpoint for; a model that a team file adds to a kept
provider without one SHALL stay unusable on that protocol until
`aigw provider edit --endpoint`. Every client's selection SHALL name one model
and one protocol that the model declares, the client speaks and the provider
serves. No global default or implicit inheritance SHALL take part in resolution,
readiness or client configuration files.

#### Scenario: A command declares a protocol that the provider lacks

- **WHEN** a command, or a team file for a provider that it adds, declares a
  model protocol for which the provider has no endpoint
- **THEN** validation SHALL reject it before any write
- **AND** the error SHALL name the model, the provider and the missing
  protocol.

#### Scenario: A selection needs an endpoint

- **WHEN** configuration persistence receives a client's selection whose
  provider has no endpoint for the selected protocol
- **THEN** it rejects the change and preserves the existing configuration file.

#### Scenario: Check every enabled client's model

- **WHEN** Claude Code and Codex are enabled with different selected models
- **THEN** `aigw check` SHALL derive each AIGW-owned authenticated request from that client's selected protocol and its
  provider's Token
- **AND** SHALL NOT inspect an unselected model as a fallback
- **AND** MAY share one authenticated request between clients only when
  provider, endpoint, protocol, depth and any carried model ID are identical.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `aigw status --json` reports a client whose provider has an endpoint
  for the selected protocol
- **THEN** it SHALL report that the endpoint is configured
- **AND** SHALL NOT represent that configuration fact as endpoint or transport
  health.
