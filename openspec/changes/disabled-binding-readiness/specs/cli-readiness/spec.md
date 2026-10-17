# Spec Delta

## ADDED Requirements

### Requirement: A disabled Client Binding cannot change readiness

Read-only readiness commands SHALL observe credential metadata only for the
Accounts that activation selects: the enabled clients' Accounts or, when no
client is enabled, the selected bindings that activation considers. A disabled
Client Binding outside that scope SHALL be reported as deferred without
observing its Account, so that Account's credential state cannot change another
client's state, the overall next action or the doctor result.

#### Scenario: A disabled binding's Account metadata is inaccessible

- **GIVEN** Codex is enabled on an Account whose Token is available
- **AND** Claude Code keeps a selected Route on another Account, but its binding
  is disabled
- **AND** that other Account's credential metadata cannot be observed
- **WHEN** `status`, `check` or `doctor` runs
- **THEN** it SHALL report the same result as when that metadata is available
- **AND** Claude Code SHALL be deferred because it is not installed or enabled
- **AND** the command SHALL NOT observe the other Account's credential metadata.

#### Scenario: Every binding is disabled

- **WHEN** no client is enabled and activation considers a selected binding
- **THEN** status SHALL keep reporting that binding's Token availability and
  its recovery action.
