## ADDED Requirements

### Requirement: Operational commands share one state vocabulary

`setup`, `use`, `sync`, `status`, `check`, `doctor`, and `verify` SHALL use
configured, deferred, endpoint_checked, degraded, invalid, and unavailable as one shared
state vocabulary. Commands SHALL classify only the evidence they actually
observe: a deeper authenticated probe may refine configured into endpoint_checked,
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
- **THEN** a successful probe reports endpoint_checked, not client or inference readiness
- **AND** a typed probe failure reports degraded, invalid, or unavailable
  without changing the underlying local configuration.

#### Scenario: A capability is intentionally deferred

- **WHEN** a Profile is present but its client is absent or its Account has not
  been connected
- **THEN** read-only commands report the exact deferred capability
- **AND** do not describe the whole installation as corrupt.

#### Scenario: Optional account diagnostics are unavailable

- **WHEN** an enabled client Route passes its configuration, projection,
  Account Token, and endpoint checks but optional balance credentials are
  unavailable
- **THEN** human and JSON check output both report that Route as endpoint_checked
- **AND** check does not access optional diagnostic credentials
- **AND** the dedicated account and balance commands retain responsibility for
  connecting and using those credentials.

#### Scenario: Recovery is unavailable

- **WHEN** no valid checkpoint, predecessor, or owned projection exists for a
  recovery command
- **THEN** the command identifies the failed recovery boundary and states that
  the current configuration or program remains the only confirmed state
- **AND** presents exactly one safe next action
- **AND** preserves the underlying storage or transaction error only as a
  diagnostic cause rather than describing its internal representation as the
  user problem.

#### Scenario: A lower-priority recovery source remains valid

- **WHEN** the preferred verified configuration is absent or invalid but the
  immediate predecessor is valid
- **THEN** configuration rollback restores that predecessor
- **AND** does not fail merely because the preferred source was unusable.

### Requirement: Endpoint observations preserve service independence

Account admission and readiness SHALL share one loopback-host classification:
case-insensitive `localhost` and standard IPv4, IPv6, and IPv4-mapped loopback
addresses. Classification SHALL use the configured address without DNS lookup,
credential access, listener probing, or service discovery. Plain HTTP SHALL
remain restricted to that loopback boundary.

#### Scenario: Both clients select loopback endpoints

- **WHEN** Claude and Codex Routes select loopback endpoints
- **THEN** human status SHALL show both clients' endpoint observations
- **AND** JSON SHALL classify the same endpoints as `external_loopback`
- **AND** neither representation SHALL infer a compatibility layer, service
  identity, listener health, or lifecycle ownership from the address.

#### Scenario: An Account selects a private non-loopback address

- **WHEN** an Account endpoint uses a private or unspecified network address
- **THEN** admission SHALL require HTTPS
- **AND** readiness SHALL NOT classify that address as loopback.

### Requirement: Verification follows enabled client scope

`verify --for all` SHALL verify every currently enabled client Route, not every
client implemented by AIGW. Disabled or absent clients SHALL NOT become
prerequisites for Account rename finalization. Finalization SHALL retain
credential equality, current-configuration and exact-backup guards; it SHALL
require successful verification of all enabled clients before retiring source
credentials. A configuration with no enabled clients needs no client checkpoint
and SHALL NOT be described as having completed client verification.

#### Scenario: One client is enabled

- **WHEN** only Claude Code or only Codex is enabled after an Account rename
- **THEN** bulk verification invokes only that enabled client and records its
  exact scope
- **AND** finalization accepts that scope without installing or invoking the
  other client.

#### Scenario: An enabled client is unready

- **WHEN** an enabled client has a missing Route or invalid projection
- **THEN** bulk verification fails before any client invocation
- **AND** no complete verification checkpoint is written.

#### Scenario: No client is enabled

- **WHEN** an Account is renamed before any client is enabled
- **THEN** finalization can converge the current backup and safely retire equal
  source credentials without requiring a client checkpoint
- **AND** bulk verification reports that there are no enabled clients rather
  than claiming an empty successful inference.

### Requirement: Read-only commands remain non-interactive

Read-only commands SHALL NOT prompt, mutate configuration, read secret values
unless authenticating a declared probe, start a client, or repair a projection.

#### Scenario: Credential metadata is inaccessible

- **WHEN** a read-only command cannot observe credential metadata
- **THEN** it reports the backend boundary and recovery action
- **AND** does not open an operating-system credential prompt.
