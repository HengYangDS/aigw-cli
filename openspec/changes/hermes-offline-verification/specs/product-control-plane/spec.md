# Spec Delta

## ADDED Requirements

### Requirement: Hermes verification excludes unrelated update services

When AIGW verifies an enabled Hermes Client Binding, it SHALL invoke the native
client against the selected Route without requiring the client's upstream
software-update service. The disposable verification environment SHALL preserve
the operator's Hermes configuration and credentials, and version-probe failures
SHALL be classified without exposing raw vendor output or private paths.

#### Scenario: Vendor update service is unavailable

- **WHEN** the selected inference endpoint is healthy but the Hermes software
  update service is unavailable
- **THEN** AIGW can observe the native client version and complete one bounded
  request for the selected Model without contacting the update service
- **AND** no user's Hermes configuration or credential is changed.

#### Scenario: Hermes version probe fails

- **WHEN** the native Hermes version probe times out or exits unsuccessfully
- **THEN** verification fails with the corresponding cause category
- **AND** it does not report successful inference, retry the probe, prompt for
  credentials, or disclose raw stderr, Token material, or private paths.
