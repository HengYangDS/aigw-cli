## MODIFIED Requirements

### Requirement: An explicit client-scoped profile is self-describing

Every Profile SHALL declare exactly one admitted client and exactly one model.
Commands that receive a Profile SHALL derive the target client from that
Profile rather than require or accept a second client selection.

#### Scenario: Connectivity test selects a Codex profile

- **WHEN** the operator runs `aigw test --profile <profile>`
- **AND** the Profile declares `client = "codex"`
- **THEN** only the Codex endpoint is tested
- **AND** no redundant client input is required.

#### Scenario: Live verification selects a Claude Code profile

- **WHEN** the operator runs `aigw verify --profile <profile>`
- **AND** the Profile declares `client = "claude"`
- **THEN** one Claude Code protocol request is verified
- **AND** no redundant client input is required.

#### Scenario: Select a Profile

- **WHEN** the operator runs `aigw use <profile>`
- **THEN** AIGW SHALL update only the Route for the Profile's declared client
- **AND** SHALL leave every other client Route unchanged.

#### Scenario: Test or verify a named Profile

- **WHEN** the operator supplies `--profile <profile>` to a connectivity or
  live-verification command
- **THEN** AIGW SHALL target only the Profile's declared client
- **AND** SHALL NOT require a redundant client argument.

## ADDED Requirements

### Requirement: Route selection has one authority

The configuration SHALL contain one explicit mapping from each selected client
to its Profile. AIGW SHALL NOT resolve a client through a global default,
inheritance, cross-client fallback, or a second override layer.

#### Scenario: Select independent Claude and Codex Profiles

- **WHEN** the operator selects one Claude Profile and one Codex Profile
- **THEN** each client SHALL resolve through its own Route
- **AND** changing either Route SHALL not change the other.

#### Scenario: A client has no selected Route

- **WHEN** an operation requires a client for which no Route is selected
- **THEN** AIGW SHALL report that exact missing Route
- **AND** SHALL recommend selecting a compatible Profile
- **AND** SHALL NOT substitute another client's Profile.

### Requirement: Previous local configuration migrates once

AIGW SHALL convert the previous default-plus-overrides schema into explicit
client Routes at the read boundary. The current runtime and persisted schema
SHALL NOT retain the previous resolution model or read both models in parallel.

#### Scenario: An override and the previous default select the same client

- **WHEN** previous configuration contains an explicit override for a client
- **AND** its previous default Profile declares that same client
- **THEN** the explicit override SHALL become that client's Route.

#### Scenario: The previous default is unambiguous

- **WHEN** the previous default Profile declares exactly one admitted client
  and model
- **AND** that client has no explicit override
- **THEN** the Profile SHALL become that client's Route.

#### Scenario: Previous Profile identity is ambiguous

- **WHEN** a previous Profile does not declare exactly one admitted client and
  model
- **THEN** migration SHALL fail before changing persisted configuration
- **AND** SHALL require explicit operator correction rather than guessing.
