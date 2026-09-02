## ADDED Requirements

### Requirement: Operational commands share one state vocabulary

`setup`, `use`, `sync`, `status`, `check`, `doctor`, and `verify` SHALL use
configured, deferred, ready, degraded, invalid, and unavailable as one shared
state vocabulary. Commands SHALL classify only the evidence they actually
observe: a deeper authenticated probe may refine configured into ready,
degraded, invalid, or unavailable. Human and JSON output SHALL identify the
affected Account, Profile, Route, client, backend, or endpoint and exactly one
safe next action.

#### Scenario: Local client prerequisites are configured

- **WHEN** its Route resolves, its Account Token is available, its Adapter
  projection matches, and the command performs no authenticated endpoint probe
- **THEN** status and doctor report that client as configured
- **AND** no unrelated unselected Account or absent client changes the result.

#### Scenario: An authenticated probe refines readiness

- **WHEN** check observes a configured client through its bounded authenticated
  endpoint probe
- **THEN** a successful probe reports ready
- **AND** a typed probe failure reports degraded, invalid, or unavailable
  without changing the underlying local configuration.

#### Scenario: A capability is intentionally deferred

- **WHEN** a Profile is present but its client is absent or its Account has not
  been connected
- **THEN** read-only commands report the exact deferred capability
- **AND** do not describe the whole installation as corrupt.

#### Scenario: Recovery is unavailable

- **WHEN** no valid checkpoint, predecessor, or owned projection exists for a
  recovery command
- **THEN** the command reports healthy absence or unavailable recovery as
  appropriate
- **AND** does not describe the internal journal representation as the user
  problem.

### Requirement: Read-only commands remain non-interactive

Read-only commands SHALL NOT prompt, mutate configuration, read secret values
unless authenticating a declared probe, start a client, or repair a projection.

#### Scenario: Credential metadata is inaccessible

- **WHEN** a read-only command cannot observe credential metadata
- **THEN** it reports the backend boundary and recovery action
- **AND** does not open an operating-system credential prompt.
