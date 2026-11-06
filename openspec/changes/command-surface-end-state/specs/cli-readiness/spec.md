## ADDED Requirements

### Requirement: Check climbs one ladder of evidence

`aigw check [client...] --depth <depth>` SHALL run, for each checked client,
every depth up to the requested one in the order `local`, `endpoint`, `model`
and `client`, with `model` as the default. Every checked client SHALL get a row
naming the deepest depth it reached and claiming nothing beyond it, and a
client SHALL stop at its first failed depth while the other clients continue.

#### Scenario: Each depth proves only its own evidence

- **WHEN** a client passes a depth
- **THEN** passing `local` SHALL mean that the configuration is valid, the
  Token is present, the client files match and no `ANTHROPIC_*` or
  `OPENAI_API_KEY` variable conflicts; `endpoint`, that the provider answered
  one authenticated request without a model; `model`, that the selected model
  answered one bounded inference request; and `client`, that a real client
  session completed, as observed for that request and client
- **AND** a present Token, matching client files or an endpoint answer SHALL
  NOT alone show that a real client can authenticate or infer, so `status`
  MAY report such a client ready without claiming either.

#### Scenario: The default depth

- **WHEN** `aigw check` runs without `--depth`
- **THEN** each enabled client SHALL run the `local`, `endpoint` and `model`
  depths, which may use provider quota
- **AND** no client executable SHALL start.

#### Scenario: A client fails at a depth

- **GIVEN** UCloud rejects Hermes's Token with HTTP 401, and Codex has no
  selected model
- **WHEN** `aigw check` runs
- **THEN** Hermes's row SHALL name `endpoint` and the rejection, with
  `aigw provider rotate ucloud` as its Fix, and Codex's row its `local` failure
- **AND** neither SHALL run a deeper depth, while the other clients continue.

#### Scenario: Local checks in detail

- **WHEN** `aigw check --depth local --verbose` runs
- **THEN** it SHALL print every local check and send no request
- **AND** `aigw check --json` SHALL carry every local check with or without
  `--verbose`.

### Requirement: A depth that a client cannot run is skipped

A depth that a checked client cannot run SHALL be skipped and reported with its
reason, and a skip SHALL NOT count as a failure. `aigw check` SHALL exit 1 when
a checked client failed, or when no client is named and none is enabled, and 0
otherwise, so named clients' own results decide.

#### Scenario: A client authenticates natively

- **WHEN** a checked client uses client-native or external-helper
  authentication
- **THEN** its `endpoint` and `model` depths SHALL be skipped with that reason
- **AND** check SHALL validate its local configuration without reading AIGW
  Tokens or client-owned credentials, and its success SHALL remain local
  configuration evidence.

#### Scenario: Claude Code keeps a proven native model preference

- **WHEN** its attributed sidecar proves that only the top-level model that
  AIGW wrote changed, while the endpoint, helper and managed credentials did
  not
- **THEN** the local check SHALL accept the connection and report the native
  model preference without rewriting it
- **AND** check SHALL skip the `model` depth, because AIGW does not choose the
  model that Claude Code sends, and SHALL name
  `aigw check claude --depth client` for native-model proof
- **AND** an endpoint, helper or managed-credential edit SHALL remain invalid.

#### Scenario: Claude Desktop is checked at the client depth

- **WHEN** `aigw check --depth client` checks Claude Desktop
- **THEN** its `client` depth SHALL be skipped, because Claude Desktop needs
  acceptance inside the app
- **AND** the command SHALL exit 0 when no checked client failed.

### Requirement: The verified snapshot records each client's depth

`aigw check --depth client` without named clients SHALL write the verified
snapshot that `aigw config rollback` restores only when no checked client
failed and every client that can run a session ran one, and the snapshot SHALL
record the depth each client reached. With no enabled client, it SHALL write no
snapshot and claim no completed client.

#### Scenario: Claude Desktop is enabled

- **GIVEN** Claude Code, Claude Desktop and Codex are enabled
- **WHEN** `aigw check --depth client` completes sessions for Claude Code and
  Codex, and Claude Desktop passes the `model` depth
- **THEN** the verified snapshot SHALL be written
- **AND** it SHALL record `client` for Claude Code and Codex and `model` for
  Claude Desktop.

#### Scenario: One client is enabled

- **WHEN** only Codex is enabled and its session completes
- **THEN** the check SHALL start only Codex
- **AND** the snapshot SHALL record exactly that scope.

#### Scenario: A client fails or clients are named

- **WHEN** an enabled client fails at any depth, or the check names its
  clients, as `aigw check codex --depth client` does
- **THEN** no verified snapshot SHALL be written.

#### Scenario: No client is enabled

- **WHEN** `aigw check --depth client` runs while no client is enabled
- **THEN** it SHALL report that no client is enabled rather than an empty
  success, write no snapshot and exit 1.

### Requirement: Readiness commands share one state vocabulary

`setup`, `use`, `sync`, `status` and `check` SHALL share the configured,
deferred, degraded, invalid and unavailable states. `aigw check` SHALL report
the depth each client reached instead of a checked state, with `client` marking
a completed session, and a bounded authenticated request SHALL report only the
depth it observed. Human and JSON output SHALL name the affected provider,
model, client, backend or endpoint and exactly one Fix.

#### Scenario: Local client prerequisites are configured

- **WHEN** a client's model resolves, its provider's Token is available, its
  client configuration files match, and the command sends no authenticated
  request
- **THEN** `status` and `aigw check --depth local` SHALL report that client as
  configured
- **AND** no provider that no client uses and no absent client SHALL change the
  result.

#### Scenario: An authenticated probe refines readiness

- **WHEN** check reaches a configured client's `endpoint` depth, including
  with `--depth endpoint`
- **THEN** its bounded authenticated request SHALL carry no model, and success
  SHALL report the `endpoint` depth, not client or inference readiness
- **AND** a typed failure SHALL report degraded, invalid or unavailable without
  changing the local configuration.

#### Scenario: An inference-scoped probe refines readiness

- **WHEN** check sends, at the `model` depth, a bounded authenticated request
  carrying the selected model's exact ID
- **THEN** success SHALL report the `model` depth, not real-client readiness or
  a guarantee of future availability
- **AND** a provider's refusal of that model SHALL report degraded and name the
  model without changing local configuration
- **AND** a model that cannot be resolved SHALL fail closed rather than become
  an endpoint-only success
- **AND** AIGW SHALL retain no probe conversation, request no storage where the
  protocol supports it, and make no claim about provider retention.

#### Scenario: A credential rejection is terminal for one diagnostic

- **WHEN** an `endpoint` or `model` request receives an HTTP 401 or 403
  credential rejection
- **THEN** check SHALL report the typed credential failure after exactly one
  request
- **AND** it SHALL NOT repeat the request, prompt for credentials or change the
  configuration.

#### Scenario: A capability is intentionally deferred

- **WHEN** a model is present but its client is absent or its provider has not
  been connected
- **THEN** read-only commands SHALL report the exact deferred capability
- **AND** SHALL NOT describe the whole installation as corrupt.

#### Scenario: A team file activated no client

- **WHEN** a team file was applied but no compatible provider Token or client
  selection has been activated
- **THEN** `status` and `check` SHALL show zero enabled clients and the
  deferred activation state in human and JSON output
- **AND** `check` without named clients SHALL exit 1 at every depth, with
  `ok: false` in one JSON document and no endpoint or inference request
- **AND** an environment-backed continuation SHALL name one compatible provider
  variable from availability metadata, without exposing its value or requiring
  every provider
- **AND** an empty `sync` selection SHALL NOT recommend `aigw check`.

#### Scenario: Optional balance credentials are unavailable

- **WHEN** a checked client passes its configuration, client file, Token and
  endpoint checks but optional balance credentials are unavailable
- **THEN** human and JSON check output SHALL both report the depth that client
  reached
- **AND** check SHALL NOT access optional balance credentials
- **AND** `aigw provider edit` and `aigw provider balance` SHALL keep
  responsibility for storing and using those credentials.

#### Scenario: Recovery is unavailable

- **WHEN** no valid verified snapshot, predecessor or AIGW-owned client
  configuration exists for a recovery command
- **THEN** the command SHALL identify the failed recovery boundary and state
  that the current configuration or program remains the only confirmed state
- **AND** SHALL present exactly one Fix
- **AND** SHALL keep the underlying storage or transaction error only as a
  diagnostic cause, not as the user problem.

#### Scenario: A lower-priority recovery source remains valid

- **WHEN** the verified snapshot is absent or invalid but the immediate
  predecessor is valid
- **THEN** `aigw config rollback` SHALL restore that predecessor
- **AND** SHALL NOT fail merely because the preferred source was unusable.

### Requirement: Check reports one machine-readable document

`aigw check --json` SHALL write one JSON document from the same evaluation as
its human output, without changing configuration, credentials, client files or
network state beyond the requests of its depths. On macOS, the installed
program's first read of a retained Keychain item MAY perform the one-time
migration to AIGW ownership, which keeps the Token and the item's service and
account attributes unchanged.

#### Scenario: Each checked client is reported as structured facts

- **WHEN** `aigw check --json` runs with valid configuration and enabled
  clients
- **THEN** it SHALL write one document that gives, for each checked client, its
  `provider/model`, its provider, the depth it reached, each skipped depth with
  its reason and every local check, and the overall result
- **AND** human `aigw check` SHALL report the same evaluation.

#### Scenario: Optional entries do not block readiness

- **WHEN** the configuration holds providers or models that no checked client
  uses and that have no Token
- **THEN** `aigw check --json` SHALL NOT require their Tokens
- **AND** the result SHALL treat only the checked clients' models as readiness
  requirements.

#### Scenario: Missing active credentials remain actionable

- **WHEN** a checked client's provider lacks its required Token
- **THEN** the command SHALL exit 1
- **AND** its JSON result SHALL name the provider and one Fix without exposing
  Token material.

### Requirement: A disabled client cannot change readiness

Read-only readiness commands SHALL observe credential metadata only for the
providers that activation selects: those of the enabled and named clients or,
when no client is enabled, of the selections that activation considers. A
disabled client outside that scope SHALL be reported as deferred without
observing its provider, so that provider's credential state cannot change
another client's state, the overall next action or the local checks.

#### Scenario: A disabled client's provider metadata is inaccessible

- **GIVEN** Codex is enabled on a provider whose Token is available
- **AND** Claude Code keeps a selected model of another provider, but is
  disabled
- **AND** that other provider's credential metadata cannot be observed
- **WHEN** `status`, or `check` without naming Claude Code, runs
- **THEN** it SHALL report the same result as when that metadata is available
- **AND** Claude Code SHALL be deferred because it is not installed or enabled
- **AND** the command SHALL NOT observe the other provider's credential
  metadata.

#### Scenario: Every client is disabled

- **WHEN** no client is enabled and activation considers a selected model
- **THEN** status SHALL keep reporting that provider's Token availability and
  its recovery action.

### Requirement: Providers and readiness share one loopback classification

Provider endpoint validation and readiness SHALL share one loopback-host
classification: case-insensitive `localhost` and standard IPv4, IPv6 and
IPv4-mapped loopback addresses. Classification SHALL use the configured address
without DNS lookup, credential access, listener probing or service discovery.
Plain HTTP SHALL remain restricted to that loopback boundary.

#### Scenario: Both clients use loopback endpoints

- **WHEN** the models that Claude Code and Codex use belong to providers with
  loopback endpoints
- **THEN** human status SHALL show both clients' endpoint observations
- **AND** JSON SHALL classify the same endpoints as `external_loopback`
- **AND** neither representation SHALL infer a compatibility layer, service
  identity, listener health or lifecycle ownership from the address.

#### Scenario: A provider endpoint uses a private non-loopback address

- **WHEN** a provider endpoint uses a private or unspecified network address
- **THEN** `aigw provider add` and `aigw provider edit` SHALL require HTTPS
- **AND** readiness SHALL NOT classify that address as loopback.

### Requirement: A real Codex session proves the Codex client depth

At the `client` depth, AIGW SHALL check Codex by running its configured Codex
executable against one synchronized Codex configuration that AIGW wrote. A
direct HTTP request made by AIGW itself MUST NOT be accepted as proof of the
Codex client path.

#### Scenario: A synchronized Codex target is checked

- **WHEN** Codex is checked at the `client` depth with an available
  executable, a synchronized target and a usable provider Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface with the selected target and model
- **AND** SHALL report the `client` depth only when the client's final
  response is exactly the bounded verification marker.

#### Scenario: Several synchronized Codex targets exist

- **WHEN** Codex has several synchronized targets
- **THEN** AIGW SHALL deterministically select one target for the live request
- **AND** SHALL NOT duplicate the quota-consuming request for equivalent
  targets.

#### Scenario: Codex verification identity is reported

- **WHEN** a Codex session succeeds
- **THEN** AIGW SHALL report the executable version and SHA-256 digest
  measured for that invocation
- **AND** SHALL NOT expose the provider's Token or model response content.

#### Scenario: Codex cannot run the selected model

- **WHEN** the executable is missing or incompatible, its client configuration
  is stale, authentication is unavailable, the request exceeds its bounded
  execution time, or the response marker is absent
- **THEN** the check SHALL fail with an actionable error scoped to that
  boundary
- **AND** SHALL NOT write client configuration, operator sessions,
  conversation history, model metadata or external gateway state.

#### Scenario: Codex verification reclaims its owned resources

- **WHEN** the Codex client succeeds, fails, exits while its descendants run,
  is interrupted, or reaches its turn deadline
- **THEN** AIGW SHALL stop the client process tree it started and remove its
  verification workspace before returning
- **AND** SHALL leave unrelated processes and protected files unchanged.

### Requirement: Local checks name the native file they inspect

When a local check reports an observation about one native client
configuration file under the user's home directory, human and JSON output SHALL
name that file by its path relative to the home directory. A file outside the
home directory SHALL NOT be named, and its human label SHALL say that it lies
outside the home directory. Local checks SHALL NOT expose an absolute or other
private path.

#### Scenario: Codex uses its default home

- **WHEN** a local check inspects a Codex configuration file at
  `~/.codex/config.toml`
- **THEN** the human check is labeled `Codex configuration ~/.codex/config.toml`
- **AND** the JSON check keeps its name `codex:target-1` and carries
  `~/.codex/config.toml` as `target`.

#### Scenario: A Codex target lies outside the home directory

- **WHEN** a Codex configuration file lies outside the home directory
- **THEN** the human check is labeled by its position and says that it lies
  outside the home directory
- **AND** neither human nor JSON output contains the file's path.

### Requirement: Local checks know a target on another volume is outside home

When the home directory and a target are absolute paths on different volumes,
local checks SHALL label the target as outside the home directory. A relative
or invalid home directory SHALL leave the target's location unknown.

#### Scenario: A Windows target is on another drive or share

- **WHEN** HOME is on one drive and the Codex target is on another drive or a
  UNC share
- **THEN** local checks SHALL label the target as outside the home directory.

#### Scenario: Volume names differ only in letter case

- **WHEN** both paths name the same drive in different letter case
- **THEN** local checks SHALL treat them as one volume.

## MODIFIED Requirements

### Requirement: Read-only commands remain non-interactive

Read-only commands SHALL NOT prompt, mutate configuration, read secret values
unless authenticating a declared probe, or rewrite a client configuration file.
They SHALL NOT start a client, except `aigw check` at the `client` depth, which
runs each client privately.

#### Scenario: Credential metadata is inaccessible

- **WHEN** a read-only command cannot observe credential metadata
- **THEN** it reports the backend boundary and recovery action
- **AND** does not open an operating-system credential prompt.

### Requirement: One-client checks do not observe unrelated clients

`aigw check` SHALL examine every enabled client when no client is named, and
`aigw check <client>...` only the named clients, enabled or not. A scoped check
SHALL NOT inspect another client's configuration files, read another
provider's credential metadata or Token, or contact another endpoint. Results
SHALL report the checked scope; JSON `clients` SHALL list only the checked
clients, and `enabled_clients` SHALL count only those that are enabled.

#### Scenario: One client is healthy while another is not ready

- **GIVEN** two enabled clients with different providers and Tokens
- **WHEN** the operator checks one of them by name
- **THEN** only that client's selected model receives the requested endpoint or
  inference request
- **AND** the other client's missing Token or client configuration cannot block
  the scoped result.

#### Scenario: The requested client is unknown or disabled

- **WHEN** `aigw check` names an unknown client
- **THEN** it SHALL refuse with the closest client names before reading
  credential metadata or contacting any endpoint
- **AND** a named disabled client SHALL be checked like an enabled one, with
  the match of its client files skipped, because it has none.

### Requirement: Hermes verification excludes unrelated update services

When `aigw check` runs Hermes at the `client` depth, it SHALL invoke the native
client with Hermes's selected model without requiring the client's upstream
software-update service. The disposable verification environment SHALL
preserve the operator's Hermes configuration and credentials, and version-probe
failures SHALL be classified without exposing raw vendor output or private
paths.

#### Scenario: Vendor update service is unavailable

- **WHEN** the selected inference endpoint is healthy but the Hermes software
  update service is unavailable
- **THEN** AIGW can observe the native client version and complete one bounded
  request for the selected model without contacting the update service
- **AND** no user's Hermes configuration or credential is changed.

#### Scenario: Hermes version probe fails

- **WHEN** the native Hermes version probe times out or exits unsuccessfully
- **THEN** verification fails with the corresponding cause category
- **AND** it does not report successful inference, retry the probe, prompt for
  credentials, or disclose raw stderr, Token material, or private paths.

### Requirement: Readiness reports an unreadable configuration's stage

`status` and `check` SHALL report a configuration that cannot be loaded with
the stage that failed, read, parse or validate, and a parse failure's line and
column, in human and JSON output, and SHALL exit 1.

#### Scenario: The configuration cannot be parsed

- **WHEN** `status` or `check` runs with `--json` and the configuration is not
  valid TOML
- **THEN** each SHALL report `configuration_parse` with the line and column
- **AND** each SHALL exit 1.

## REMOVED Requirements

### Requirement: Endpoint observations preserve service independence

**Reason**: Accounts become providers.

**Migration**: Read "Providers and readiness share one loopback
classification" in this capability, with every scenario.

### Requirement: Verification follows enabled client scope

**Reason**: `aigw verify --for all` and Account rename finalization are
removed; providers are not renamed.

**Migration**: Read "The verified snapshot records each client's depth" in
this capability for the enabled-client scope and the case of no enabled
client.

### Requirement: A disabled Client Binding cannot change readiness

**Reason**: Client Bindings become client selections, and `aigw doctor` is
removed.

**Migration**: Read "A disabled client cannot change readiness" in this
capability.

### Requirement: Readiness has one read-only machine-readable projection

**Reason**: The document's fields follow the new words and the depths.

**Migration**: Read "Check reports one machine-readable document" in this
capability; client-native authentication moves to "A depth that a client
cannot run is skipped".

### Requirement: Operational commands share one readiness state vocabulary

**Reason**: `aigw doctor`, `aigw verify` and `--endpoint-only` are removed,
the reached depth replaces the endpoint_checked and inference_checked states,
and check without named clients now exits 1 at every depth when no client is
enabled.

**Migration**: Read "Readiness commands share one state vocabulary" in this
capability; the Claude Code model preference moves to "A depth that a client
cannot run is skipped".

### Requirement: Real Codex client route verification

**Reason**: `aigw verify` becomes the `client` depth of `aigw check`.

**Migration**: Read "A real Codex session proves the Codex client depth" in
this capability.

### Requirement: Readiness observations remain separated from client execution

**Reason**: The evidence ladder owns this separation.

**Migration**: Read "Check climbs one ladder of evidence" in this capability.

### Requirement: Diagnostics name the native file they inspect

**Reason**: `aigw doctor` becomes the local checks of `aigw check`.

**Migration**: Read "Local checks name the native file they inspect" in this
capability.

### Requirement: Doctor knows a target on another volume is outside home

**Reason**: `aigw doctor` becomes the local checks of `aigw check`.

**Migration**: Read "Local checks know a target on another volume is outside
home" in this capability.
