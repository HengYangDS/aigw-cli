## MODIFIED Requirements

### Requirement: Operational commands share one state vocabulary

`setup`, `use`, `sync`, `status`, `check`, `doctor`, and `verify` SHALL use
configured, deferred, endpoint_checked, inference_checked, degraded,
invalid, and unavailable as one shared state vocabulary. Commands SHALL
classify only the evidence they actually observe: a bounded authenticated
probe may refine configured into endpoint_checked, inference_checked,
degraded, invalid, or unavailable. Human and JSON output SHALL identify the
affected Account, Route, client, backend, or endpoint and exactly one safe
next action.

#### Scenario: Local client prerequisites are configured

- **WHEN** its Route resolves, its Account Token is available, its Adapter
  projection matches, and the command performs no authenticated endpoint probe
- **THEN** status and doctor report that client as configured
- **AND** no unrelated unselected Account or absent client changes the result.

#### Scenario: An authenticated probe refines readiness

- **WHEN** check observes a configured client through a bounded authenticated
  probe that does not carry a model
- **THEN** a successful probe reports endpoint_checked, not client or inference readiness
- **AND** a typed probe failure reports degraded, invalid, or unavailable
  without changing the underlying local configuration.

#### Scenario: An inference-scoped probe refines readiness

- **WHEN** check sends a bounded authenticated request carrying the selected
  Route's exact upstream model
- **THEN** success reports inference_checked and the inference scope, not
  real-client readiness or a guarantee of future availability
- **AND** a distributor-level refusal of that model reports degraded and
  identifies the Route and model without changing local configuration
- **AND** an unresolvable upstream model fails closed rather than becoming an
  endpoint-only success
- **AND** AIGW retains no probe conversation, requests no storage where the
  protocol supports it, and makes no claim about provider retention.

#### Scenario: An operator selects endpoint-only scope

- **WHEN** check runs with --endpoint-only
- **THEN** it makes no model-carrying inference request
- **AND** a successful authenticated diagnostic reports endpoint_checked and
  the endpoint scope.

#### Scenario: Claude Code keeps a proven native model preference

- **WHEN** its attributed sidecar proves that only the projected top-level
  model changed while the endpoint, helper, and managed credentials did not
- **THEN** local inspection accepts the connection and identifies the native
  model preference without rewriting it
- **AND** check performs at most endpoint scope for that client even when
  inference scope is the command default
- **AND** check reports the actual scope and directs native-model proof to
  `aigw verify --for claude`
- **AND** an endpoint, helper, or managed-credential edit remains invalid.

#### Scenario: A capability is intentionally deferred

- **WHEN** a Route is present but its client is absent or its Account has not
  been connected
- **THEN** read-only commands report the exact deferred capability
- **AND** do not describe the whole installation as corrupt.

#### Scenario: Optional account diagnostics are unavailable

- **WHEN** an enabled client Route passes its configuration, projection,
  Account Token, and endpoint checks but optional balance credentials are
  unavailable
- **THEN** human and JSON check output both report that Route in the state
  supported by the performed diagnostic scope
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
