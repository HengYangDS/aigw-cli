# Spec Delta

## ADDED Requirements

### Requirement: Hermes verification reuses installed dependency state

When the configured Hermes home's data root holds the installation's committed
dependency environment and managed tools, AIGW's disposable Hermes verification
environment SHALL reuse that installation state. Verification SHALL NOT install,
synchronize, rebuild, or republish that state, and SHALL fail before starting
Hermes when the state cannot be shared.

#### Scenario: Hermes keeps installation state under its data root

- **WHEN** the installed Hermes resolves its committed dependency environment
  and managed tools from the data root of the configured Hermes home
- **THEN** AIGW observes the native client version and completes one bounded
  request from a disposable home that reuses that installation state
- **AND** verification refuses on-demand installation, so the shared
  dependencies, tools, and launchers are not synchronized or republished
- **AND** removing the disposable home leaves the operator's Hermes
  configuration, installation state, and launchers unchanged.

#### Scenario: Installation state cannot be shared

- **WHEN** the host refuses to expose the configured home's installation state
  to the disposable home
- **THEN** verification fails before starting Hermes
- **AND** it does not provision another dependency environment, retry, or
  disclose private paths.
