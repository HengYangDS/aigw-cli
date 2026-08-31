## MODIFIED Requirements

### Requirement: An explicit client-scoped profile is self-describing

Every Profile SHALL declare exactly one admitted client and exactly one model.
Commands that receive a Profile SHALL derive the target client from that
Profile rather than require or accept a second client selection. Selecting a
Profile SHALL converge the selected client's available adapter and projection
in the same transaction without enabling or changing another client.

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
- **AND** SHALL converge that client's available adapter and projection
- **AND** SHALL leave every other client Route and adapter unchanged.

#### Scenario: Test or verify a named Profile

- **WHEN** the operator supplies `--profile <profile>` to a connectivity or
  live-verification command
- **THEN** AIGW SHALL target only the Profile's declared client
- **AND** SHALL NOT require a redundant client argument.
