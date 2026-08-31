## MODIFIED Requirements

### Requirement: Onboarding state is explicit and actionable

Human and JSON results SHALL distinguish imported Accounts and Profiles,
connected Accounts, selected Routes, configured clients, and deferred work.
Every incomplete state SHALL expose one smallest safe next action. When no
Account is connected, that action SHALL identify one Account to connect rather
than imply that every catalogue Account is mandatory. After an Account Token or
client becomes available, setup guidance SHALL direct the operator to
`aigw sync`; `aigw check` SHALL remain verification of already enabled client
Routes, not an activation command. The setup command SHALL accept `--json` for
manifest-based setup and SHALL derive both representations from one semantic
result without exposing credential material.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  profile without inferring the endpoint implementation.

#### Scenario: No Token is available through a writable backend

- **WHEN** a valid team manifest is imported and no Account is connected
- **THEN** setup SHALL recommend `aigw rotate <account>`
- **AND** SHALL NOT imply that every catalogue Account Token is required.

#### Scenario: No Token is available through the environment backend

- **WHEN** a valid team manifest is imported with the read-only environment
  backend and no Account Token is available
- **THEN** setup SHALL name the environment variable for one Account as an
  example activation choice
- **AND** SHALL direct the operator to run `aigw sync` after setting it
- **AND** SHALL state that any one compatible Account is sufficient.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Profile counts,
  connected Accounts, client states, and next safe action as machine-readable
  data
- **AND** SHALL NOT include an Account Token or credential value.
