## MODIFIED Requirements

### Requirement: Onboarding state is explicit and actionable

Human and JSON results SHALL distinguish imported Accounts and Profiles,
connected Accounts, selected Routes, configured clients, and deferred work.
Every incomplete state SHALL expose one smallest safe next action. The setup
command SHALL accept `--json` for manifest-based setup and SHALL derive both
representations from one semantic result without exposing credential material.

#### Scenario: Selected Responses endpoint is unavailable

- **WHEN** an installed Codex route selects a Responses endpoint that is not
  reachable
- **THEN** diagnostics SHALL identify the configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than endpoint
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another Responses
  profile without inferring the endpoint implementation.

#### Scenario: Manifest setup is consumed by automation

- **WHEN** an operator runs manifest-based setup with `--json`
- **THEN** setup SHALL return the imported Account and Profile counts,
  connected Accounts, client states, and next safe action as machine-readable
  data
- **AND** SHALL NOT include an Account Token or credential value.
