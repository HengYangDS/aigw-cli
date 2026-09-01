## MODIFIED Requirements

### Requirement: Onboarding state is explicit and actionable

Human and JSON results SHALL distinguish imported Accounts and Profiles,
connected Accounts, selected Routes, configured clients, and deferred work.
Every incomplete state SHALL expose the smallest safe next action. When no
Account is connected through the environment backend, setup SHALL enumerate
the environment variables for all compatible Accounts as alternative choices
and state that any one is sufficient; it SHALL NOT present an arbitrary first
Account as the default. After an Account Token or client becomes available,
setup guidance SHALL direct the operator to `aigw sync`; `aigw check` SHALL
remain verification of already enabled client Routes, not an activation
command. The setup command SHALL accept `--json` for manifest-based setup and
SHALL derive both representations from one semantic result without exposing
credential material.

#### Scenario: Guided setup completes before client installation

- **WHEN** guided setup connects an Account while neither Claude Code nor Codex
  is installed
- **THEN** setup SHALL identify `aigw sync` as the next action after installing
  a supported client
- **AND** SHALL NOT identify `aigw check` as the activation action.

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
- **THEN** setup SHALL enumerate the environment variable for every compatible
  Account as an alternative activation choice
- **AND** SHALL direct the operator to run `aigw sync` after setting one
- **AND** SHALL state that any one compatible Account is sufficient.

#### Scenario: Environment Account becomes available later

- **WHEN** a team catalogue was imported without a connected Account
- **AND** exactly one compatible Account Token later becomes available through
  the environment backend
- **THEN** `aigw sync` SHALL activate Routes compatible with that Account
- **AND** SHALL NOT require the previously recommended Account or any unrelated
  Account Token.

#### Scenario: Connected Account precedes client installation

- **WHEN** setup connects an Account and no admitted client is installed
- **THEN** its human and machine-readable results SHALL identify `aigw sync` as
  the next action after client installation
- **AND** SHALL NOT identify an observational command as the activation action.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Profile counts,
  connected Accounts, client states, alternative activation choices, and next
  safe action as machine-readable data
- **AND** SHALL NOT include an Account Token or credential value.
