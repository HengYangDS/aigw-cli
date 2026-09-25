# cli-readiness Specification

## Purpose

Provide one read-only readiness contract for active AIGW client routes, with a
stable machine-readable projection and the same evaluation used by the human
health check.

## Requirements

### Requirement: Readiness has one machine-readable projection

The readiness command SHALL accept `--json` and emit a stable JSON document without mutating configuration, credentials, client files, or network state beyond the existing read-only diagnostics.

#### Scenario: Configured routes are reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled client routes
- **THEN** it emits one JSON document containing each enabled route's client, selected route, account, endpoint readiness, adapter readiness, and overall result
- **AND** the command uses the same readiness evaluation as human-readable `aigw check`

#### Scenario: Optional catalogue entries do not block readiness

- **WHEN** the configuration contains unselected Accounts or Routes without Tokens
- **THEN** `aigw check --json` does not require their Tokens
- **AND** the result identifies only active routes as readiness requirements

#### Scenario: Missing active credentials remain actionable

- **WHEN** an enabled Account-Token route lacks its required Token
- **THEN** the command returns a non-zero exit status
- **AND** its JSON result identifies the missing Account and a safe next action without exposing Token material

#### Scenario: Client-native authentication is selected

- **WHEN** an enabled Route uses client-native authentication
- **THEN** check SHALL validate its local projection without reading AIGW Tokens
  or client-owned credentials
- **AND** its successful result SHALL remain local configuration evidence rather
  than authenticated endpoint or model evidence.

### Requirement: Operational commands share one state vocabulary

`setup`, `use`, `sync`, `status`, `check`, `doctor`, and `verify` SHALL share
configured, deferred, endpoint_checked, inference_checked, degraded, invalid,
and unavailable states. A bounded authenticated probe SHALL report only its
observed scope. Human and JSON output SHALL identify the affected Account,
Route, client, backend, or endpoint and exactly one safe next action.

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

#### Scenario: A credential rejection is terminal for one diagnostic

- **WHEN** an endpoint-only or inference-scoped request receives an HTTP 401 or 403
  credential rejection
- **THEN** check reports the typed credential failure after exactly one request
- **AND** it does not repeat the request, prompt for credentials, or mutate
  configuration.

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

#### Scenario: A catalogue has no enabled client

- **WHEN** a reviewed manifest has been imported but no compatible Account
  Token or Client Binding has been activated
- **THEN** `status`, `check`, and `doctor` SHALL expose zero enabled clients
  and the deferred activation state in human and JSON output
- **AND** `check` SHALL return a nonzero status and `ok: false` in one JSON
  document without making an endpoint or inference request
- **AND** `doctor` MAY return success only for the explicitly named local
  diagnostic scope; it SHALL not imply that any client can infer
- **AND** an environment-backed continuation SHALL name one compatible
  Account variable using availability metadata, without exposing its value or
  requiring every Account
- **AND** an empty `sync` selection SHALL not recommend `aigw check`.

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

`verify --for all` SHALL verify exactly the enabled client Routes. Disabled or
absent clients SHALL NOT block Account rename. Finalization SHALL preserve
credential equality, current-configuration, and exact-backup guards, and retire
source credentials only after every enabled client verifies successfully. With
no enabled clients, AIGW SHALL create neither a client checkpoint nor a
completed-client claim.

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
