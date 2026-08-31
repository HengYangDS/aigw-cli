## ADDED Requirements

### Requirement: Readiness has one machine-readable projection

The readiness command SHALL accept `--json` and emit a stable JSON document without mutating configuration, credentials, client files, or network state beyond the existing read-only diagnostics.

#### Scenario: Configured routes are reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled client routes
- **THEN** it emits one JSON document containing each enabled route's client, selected profile, account, endpoint readiness, adapter readiness, and overall result
- **AND** the command uses the same readiness evaluation as human-readable `aigw check`

#### Scenario: Optional catalogue entries do not block readiness

- **WHEN** the configuration contains unselected Accounts or Profiles without Tokens
- **THEN** `aigw check --json` does not require their Tokens
- **AND** the result identifies only active routes as readiness requirements

#### Scenario: Missing active credentials remain actionable

- **WHEN** an enabled route lacks its Account Token
- **THEN** the command returns a non-zero exit status
- **AND** its JSON result identifies the missing Account and a safe next action without exposing Token material
