# Spec Delta

## ADDED Requirements

### Requirement: Provider preference is advisory control-plane input

A recommendation MAY order compatible Routes for an unselected Client Binding.
The order SHALL be declared, stable, and visible. A Route absent from that
recommendation remains available to explicit client-scoped selection but SHALL
NOT enter automatic onboarding selection. An explicit selection SHALL outrank
all recommendations until its owner changes it.

#### Scenario: A preferred Account is unavailable before activation

- **WHEN** an unselected client has several recommended compatible Routes and
  only a lower-ranked Account Token is available
- **THEN** setup or sync MAY select that usable Route without requiring the
  higher-ranked Account
- **AND** SHALL report the choice and preserve all unrelated client bindings.

#### Scenario: A manual-only provider is configured

- **WHEN** an Account and Route exist outside every recommendation for a client
- **THEN** AIGW SHALL retain them for explicit use
- **AND** SHALL NOT select them merely because another provider is unavailable.

#### Scenario: A selected provider fails during a request

- **WHEN** a client or an external endpoint reports a transient request failure
- **THEN** AIGW SHALL NOT claim to have observed or retried that request
- **AND** SHALL NOT silently rewrite the selected Route or replay the call
- **AND** MAY present a tested compatible alternative for explicit switching.

### Requirement: Ambiguous client protocols require an explicit choice

When a Route and Client Binding admit more than one endpoint protocol, AIGW SHALL
preserve an existing explicit protocol or require the operator to choose one.
It SHALL NOT infer a protocol from the Model name or silently prefer an
endpoint. Direct setup and Route selection SHALL expose that choice through
the same protocol names used by the team manifest.

#### Scenario: Hermes selects a multi-protocol Route

- **WHEN** Hermes selects a Route with multiple compatible protocols and no
  prior or recommended protocol applies
- **THEN** non-interactive selection names the required `--protocol` choice
- **AND** interactive selection offers each compatible protocol
- **AND** a failed or incompatible choice leaves every Client Binding unchanged.
