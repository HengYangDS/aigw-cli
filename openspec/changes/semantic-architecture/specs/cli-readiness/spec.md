## ADDED Requirements

### Requirement: Readiness has one read-only machine-readable projection

The readiness command SHALL accept `--json` and emit a stable JSON document without mutating configuration, credentials, client files, or network state beyond the existing read-only diagnostics. On macOS, the installed program's first read of a retained Keychain item MAY perform the one-time migration to AIGW ownership, which keeps the Token, service and Account unchanged.

#### Scenario: Configured routes are reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled client routes
- **THEN** it emits one JSON document containing each enabled route's client, selected route, account, endpoint readiness, adapter readiness, and overall result
- **AND** the command uses the same readiness evaluation as human-readable `aigw check`

#### Scenario: Optional catalog entries do not block readiness

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

### Requirement: Operational commands share one readiness state vocabulary

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

#### Scenario: A catalog has no enabled client

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

### Requirement: Real Codex client route verification

AIGW SHALL verify a selected Codex Route by running the configured and
admitted Codex executable against one synchronized AIGW projection. A direct
HTTP request made by AIGW itself MUST NOT be accepted as proof of the Codex
client path.

#### Scenario: A synchronized Codex target is verified

- **WHEN** an operator verifies a Codex Route with an available executable,
  synchronized target, and usable Account Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface using the selected target and Route model
- **AND** SHALL accept the verification only when the client's final response is
  exactly the bounded verification marker

#### Scenario: Several synchronized Codex targets exist

- **WHEN** an operator verifies a Codex Route whose adapter owns several
  synchronized targets
- **THEN** AIGW SHALL deterministically select one target for the live request
- **AND** SHALL NOT duplicate the quota-consuming request for equivalent targets

#### Scenario: Codex verification identity is reported

- **WHEN** a Codex verification succeeds
- **THEN** AIGW SHALL report the executable version and SHA-256 digest measured
  for that invocation
- **AND** SHALL NOT expose the Account Token or model response content

#### Scenario: Codex cannot execute the selected route

- **WHEN** the executable is missing or incompatible, its projection is stale,
  authentication is unavailable, the request exceeds its bounded execution
  time, or the response marker is absent
- **THEN** verification SHALL fail with an actionable error scoped to that
  boundary
- **AND** SHALL NOT write client configuration, operator sessions, conversation
  history, model metadata, or external gateway state

#### Scenario: Codex verification reclaims its owned resources

- **WHEN** the Codex client succeeds, fails, exits while its descendants run, is
  interrupted, or reaches its turn deadline
- **THEN** AIGW SHALL stop the client process tree it started and remove its
  verification workspace before returning
- **AND** SHALL leave unrelated processes and protected files unchanged

### Requirement: Verification checkpoints remain bound to current configuration

The configuration Store SHALL own checkpoint admission, bounded locking,
guarded persistence, and compensation; live requests SHALL run outside the lock.
Verification SHALL create a checkpoint only while its configuration remains
current and SHALL NOT create or certify missing configuration. Reads SHALL
consume one complete JSON document. Creation and reading SHALL share one
nonempty, distinct admitted-client scope.

#### Scenario: Invalid checkpoint content or client scope

- **WHEN** a stored checkpoint includes an additional JSON value or trailing
  malformed content, or its scope is empty, repeated or unadmitted
- **THEN** reading SHALL fail rather than certify the valid prefix
- **AND** creation with invalid scope SHALL preserve configuration, backup and
  the previous checkpoint.

#### Scenario: Configuration changes before verification completes

- **WHEN** a live verification completes after configuration has changed
- **THEN** checkpoint admission fails with an instruction to verify again
- **AND** current configuration and any newer checkpoint remain unchanged.

#### Scenario: An external editor changes configuration during checkpoint commit

- **WHEN** the Store observes changed configuration after checkpoint persistence
- **THEN** it compensates only its unchanged checkpoint postimage
- **AND** it preserves an intervening checkpoint replacement and reports any
  incomplete compensation.

#### Scenario: Checkpoint admission is cancelled

- **WHEN** the command is cancelled while waiting for the mutation lock
- **THEN** checkpoint admission returns the cancellation without a checkpoint
- **AND** the other mutation keeps ownership of its lock.

#### Scenario: A verified recovery snapshot is requested

- **WHEN** a caller requests current configuration with its verified checkpoint
- **THEN** the configuration Store SHALL establish that their persisted meanings
  match before returning verified recovery state
- **AND** formatting-only differences SHALL preserve the exact captured bytes
- **AND** a mismatch SHALL fail without changing configuration, backup or
  checkpoint, without relying on a downstream caller to repeat the check.

### Requirement: Readiness observations remain separated from client execution

AIGW SHALL distinguish local configuration readiness, endpoint observations
and real-client verification. Status SHALL observe selection and owned
projections without reading Token values or invoking native clients. A present
Token, synchronized projection or successful endpoint probe SHALL NOT alone
certify that a real client can authenticate or perform inference.

#### Scenario: A synchronized projection has no live-client evidence

- **WHEN** a client has a valid AIGW-owned projection
- **THEN** status may report that projection as ready
- **AND** does not claim native authentication or inference has been proved.

#### Scenario: Real-client verification is requested

- **WHEN** the operator invokes `aigw verify --for <client>`
- **THEN** the admitted adapter invokes that selected native client
- **AND** reports evidence bounded to the request and client it observed.

### Requirement: Recovery execution and presentation are independent

Synchronization and repair SHALL select writes through `--dry-run` and output
format through `--json` independently. Successful preview and applied results
SHALL expose their execution mode and the same continuation as human output.
Rendering SHALL NOT own transaction commit or compensation.

#### Scenario: Preview a recovery operation as JSON

- **WHEN** `sync` or `repair` succeeds with `--dry-run --json`
- **THEN** output SHALL be one JSON document with `dry_run: true`
- **AND** `next_action` SHALL identify the corresponding apply command
- **AND** configuration, credentials and client projections SHALL remain
  unchanged.

#### Scenario: Apply a recovery operation with JSON output

- **WHEN** `sync` or `repair` succeeds with `--json` without `--dry-run`
- **THEN** the owned projection SHALL be applied
- **AND** output SHALL be one JSON document with `dry_run: false` and
  `next_action: "aigw check"`
- **AND** preview plans SHALL NOT be presented as observed per-target results.

#### Scenario: Output fails after a successful recovery commit

- **WHEN** a recovery operation commits and its result cannot be written
- **THEN** it SHALL return the output error and preserve the committed state
- **AND** generic presentation SHALL NOT claim that rollback occurred or will
  occur.

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

### Requirement: Hermes verification reuses installed dependency state

When the configured Hermes home's data root holds the installation's committed
dependency environment and managed tools, AIGW's disposable Hermes verification
environment SHALL reuse them without borrowing mutable home-maintenance records,
and its own boot records SHALL remain disposable. The probe SHALL NOT activate
operator plugins or credential-backed memory providers, or install, synchronize,
rebuild, or republish the operator's dependencies, tools, launchers, or boot
records.

#### Scenario: Verification keeps the native dependency selection

- **WHEN** the disposable environment reuses the committed dependencies
- **THEN** native dependency selection SHALL retain its original physical lock
  and generation leases
- **AND** SHALL include the complete native live-profile dependency-member
  union with its original resolved identities.

#### Scenario: Hermes keeps installation state under its data root

- **WHEN** the installed Hermes resolves its committed dependency environment
  and managed tools from the data root of the configured Hermes home
- **THEN** AIGW observes the native client version and completes one bounded
  request from a disposable home outside the operator's native data roots
- **AND** verification refuses on-demand installation, so the installed
  dependencies, tools, and launchers are not synchronized or republished
- **AND** removing the disposable home leaves the operator's Hermes
  configuration, installation state, and launchers unchanged.

#### Scenario: Installation state cannot be shared

- **WHEN** the host refuses to expose the configured home's installation state
  to the disposable home
- **THEN** verification fails before starting Hermes
- **AND** it does not provision another dependency environment, retry, or
  disclose private paths.

#### Scenario: Installed dependencies include native plugin members

- **WHEN** default or live named profiles contribute plugin or memory-provider
  members to the committed dependency environment
- **THEN** disposable selector-only profiles retain their native names,
  disabled selections and resolved source identities without copying credentials
- **AND** retired profiles, invalid profile names and the default alias do not
  change the dependency union
- **AND** the inference probe disables plugin and memory-provider activation
  while native dependency and boot-record checks remain effective.

#### Scenario: Native launch effects cannot be isolated

- **WHEN** the installed platform's boot path cannot preserve the operator's
  launchers or safely borrow its committed dependency selection
- **THEN** AIGW refuses verification before starting the client
- **AND** it does not invent a private selection lock, provision dependencies,
  modify client configuration, or claim readiness
- **AND** the refusal discloses no credential diagnostics.

#### Scenario: Required installation state is unavailable

- **WHEN** the installed Hermes cannot resolve required dependency state from
  the configured native root
- **THEN** verification reports the native failure without claiming readiness
- **AND** it does not provision another dependency environment, retry, or
  disclose private paths.

### Requirement: Hermes verification releases its gateway before deleting its home

Hermes verification SHALL bound the idle lifetime of the gateway its chat
starts, keep that gateway's host locks inside the verification home, and
delete the home only after observing that no process holds its gateway lock.
When the lock stays held after one native stop request, verification SHALL
retain the home, name it, and report incomplete verification.

#### Scenario: The verification gateway exits after the chat

- **WHEN** the verification chat returns and its gateway exits on idle
- **THEN** verification SHALL delete its home only after the gateway lock is
  free
- **AND** the operator's host lock directory SHALL remain untouched.

#### Scenario: The verification gateway outlives its idle bound

- **WHEN** the gateway lock is still held after the idle bound
- **THEN** verification SHALL request `hermes gateway stop` once with closed
  input for that home and wait again
- **AND** it SHALL send no signal to the gateway.

#### Scenario: The verification gateway keeps running

- **WHEN** the gateway lock is still held after the stop request
- **THEN** verification SHALL retain the home and name it in its error
- **AND** it SHALL NOT report completed verification.

### Requirement: Hermes verification waits for its gateway process to exit

Hermes verification SHALL delete its disposable home only after the gateway
process that the home's `gateway_state.json` records has exited, in addition to
the free gateway lock. Hermes releases that lock before it writes its exit
records and logs into the home, and keeps the state file after the gateway
stops. A process that outlives the bound SHALL leave the home in place and
named in the error.

#### Scenario: The gateway releases its lock before it exits

- **WHEN** the gateway lock is free but the recorded gateway process still runs
- **THEN** verification SHALL wait for that process to exit before it deletes
  the home
- **AND** verification SHALL keep the home and name it in the error when the
  process outlives the bound.

#### Scenario: No gateway recorded a run

- **WHEN** the home has no gateway state file
- **THEN** the free gateway lock alone SHALL decide whether the home is deleted.

### Requirement: The Hermes release journey resumes its session natively

The native release journey SHALL create its Hermes session once with its
creation options, resume it by the stored session identifier without options
that Hermes freezes at creation, and stop the journey's gateway before
deleting the journey home.

#### Scenario: Continue the Hermes session across program lifecycle steps

- **WHEN** the journey continues its Hermes session after an update, rollback
  or re-upgrade
- **THEN** it SHALL pass `--resume` with the stored session identifier
- **AND** Hermes SHALL retain the earlier tool result in the continued input.

## REMOVED Requirements

### Requirement: Readiness has one machine-readable projection

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Readiness has one read-only machine-readable projection" in this capability, with every scenario.

### Requirement: Operational commands share one state vocabulary

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Operational commands share one readiness state vocabulary" in this capability, with every scenario.
