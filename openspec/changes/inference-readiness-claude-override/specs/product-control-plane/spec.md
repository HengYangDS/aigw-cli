## MODIFIED Requirements

### Requirement: Route and binding compatibility

Every Route SHALL reference one existing Account and one canonical Model and
SHALL declare its admitted protocol interfaces. Every Client Binding SHALL
select one compatible Route and one protocol admitted by both that Route and
the client.

#### Scenario: Import a Route with an incompatible Account

- **WHEN** a Route declares a protocol interface but its Account lacks that
  protocol endpoint
- **THEN** configuration validation and manifest import reject it before writes
- **AND** the error identifies the Route, Account, and missing protocol.

#### Scenario: Save an incompatible Route

- **WHEN** configuration persistence receives a Route whose Account lacks its
  declared protocol endpoint
- **THEN** it rejects the change and preserves the existing configuration file.

#### Scenario: Select independent Claude and Codex services

- **WHEN** an operator selects a Claude Route and a Codex Route
- **THEN** each client SHALL retain its own explicit Route
- **AND** no global default or implicit inheritance SHALL participate in
  resolution, readiness, or projection.

#### Scenario: Interactive selection omits the client

- **WHEN** an interactive operator runs `aigw use <route>` without `--for`
- **THEN** AIGW SHALL offer only admitted clients compatible with that Route
- **AND** non-interactive invocation SHALL require `--for <client>` rather than
  infer a client from the Route.

#### Scenario: Check every enabled client Route

- **WHEN** Claude and Codex are enabled with distinct selected Routes
- **THEN** `aigw check` SHALL validate both effective Routes
- **AND** SHALL derive each AIGW-owned authentication request from that Client Binding's
  selected protocol and authentication mode
- **AND** SHALL not issue an AIGW-owned authentication request for a
  client-native Route
- **AND** SHALL not inspect an unselected historical Route as a fallback
- **AND** MAY coalesce authenticated probes only when Account, endpoint,
  protocol, performed scope, and any carried upstream model are identical.

#### Scenario: Check a client-native Route

- **WHEN** an enabled Route selects a client-native Route whose local client
  projection is valid
- **THEN** `aigw check` SHALL report local readiness without claiming that the
  remote authentication or model request succeeded
- **AND** SHALL provide `aigw verify --for <client>` as the explicit live-proof
  continuation.

#### Scenario: Claude keeps a sidecar-proven native model preference

- **WHEN** Claude Code changes only its top-level native model after AIGW
  projects a Route, endpoint, and credential helper
- **THEN** local inspection accepts the unchanged connection without treating
  the native alias as the Route's upstream wire model
- **AND** `aigw check` uses no model-carrying AIGW request for that client,
  reports at most endpoint_checked with endpoint diagnostic scope, and provides
  `aigw verify --for claude` as the native-model continuation
- **AND** Codex and Claude Desktop retain their own selected Routes and probe
  scopes.

#### Scenario: No client is enabled

- **WHEN** configuration is valid but no admitted client Adapter is enabled
- **THEN** `aigw check` SHALL report deferred activation and return nonzero
- **AND** SHALL not claim that an arbitrary gateway or model is healthy.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `status --json` reports a resolved Route with an endpoint address
- **THEN** it SHALL report `endpoint_configured` as true
- **AND** SHALL not represent that configuration fact as endpoint or transport
  health.

#### Scenario: Report the exact scope of a successful check

- **WHEN** `check --json` finishes its applicable Route checks
- **THEN** `check_passed` SHALL describe command-check success, not inference
  or real-client readiness
- **AND** a valid client-native projection SHALL remain `configured` without
  an AIGW-owned endpoint diagnostic
- **AND** a configured Account-Token Route SHALL report the state its performed
  diagnostic scope supports: `endpoint_checked` for a successful model-free
  endpoint diagnostic and `inference_checked` for a successful diagnostic that
  carried that Route's upstream model, never generically `ready`
- **AND** `check --json` SHALL report the performed diagnostic scope, including
  an endpoint-only observation selected because Claude has a proven native
  model preference
- **AND** human output SHALL describe the same observed scope
- **AND** `next_action` SHALL carry both repair and verification continuations
  without a duplicate `fix` field.

#### Scenario: Read previous local configuration

- **WHEN** AIGW reads its local configuration
- **THEN** it SHALL accept the current schema with explicit per-client Routes
- **AND** an earlier schema SHALL return its actual version and the current
  required version without inferring or migrating Route selection.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** a team manifest declares recommended Routes for admitted clients
- **THEN** setup SHALL retain those recommendations separately from actual
  per-client selections and fill only currently usable unselected Routes
- **AND** neither a global default nor an unavailable recommendation SHALL
  replace an explicit client selection.
- **AND** a future client SHALL remain unselected until its own Route is
  explicitly admitted.

#### Scenario: Diagnose a partially connected team catalogue

- **WHEN** a reviewed catalogue contains multiple Accounts
- **AND** every Account selected by an enabled client Route has its Token
- **THEN** `aigw doctor` SHALL report the credential state as healthy
- **AND** SHALL NOT fail for an unselected Account whose Token is absent.

#### Scenario: Diagnose a selected Account without a Token

- **WHEN** an enabled client Route selects an Account-Token Route whose Token
  is absent
- **THEN** `aigw doctor` SHALL report that Account as unhealthy
- **AND** SHALL provide the account-scoped rotation action.

#### Scenario: Diagnostic outcome is independent of presentation

- **WHEN** `doctor` observes a failed diagnostic or an invalid, degraded,
  unavailable or unclassified admitted-client state
- **THEN** human and JSON output SHALL both return a nonzero exit status
- **AND** JSON SHALL contain `ok: false` and the same `next_action` shown to
  the human reader
- **AND** JSON SHALL remain one document without appended prose or another
  error rendering
- **AND** successfully writing that report SHALL NOT change the diagnosis.

#### Scenario: Deferred clients do not invalidate a sound configuration

- **WHEN** all diagnostics pass and the observed clients are configured,
  endpoint-checked or intentionally deferred
- **THEN** `doctor` SHALL return success in human and JSON modes
- **AND** SHALL not invent a repair continuation.

#### Scenario: Diagnose a client-native Route

- **WHEN** an enabled client Route selects a client-native Route
- **THEN** `status`, `check`, `doctor`, `test`, `credential`, and Route
  inspection SHALL NOT query the AIGW Account Token store for that Route
- **AND** read-only output SHALL identify client-owned authentication and use
  `aigw verify --for <client>` for live proof where applicable.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Route data for a new
  endpoint
- **THEN** Codex or Claude Code SHALL select it without a provider-specific CLI,
  installer, projection branch, service manager, or core dependency.

#### Scenario: A Responses endpoint needs compatibility behavior

- **WHEN** an endpoint needs storage, replay, or other wire compatibility
- **THEN** AIGW SHALL NOT rename its provider identity or encode transport
  behavior in Account metadata.

#### Scenario: Reject implicit credential transport

- **WHEN** an imported manifest contains a token, password, authorization
  header, API key, or equivalent credential field
- **THEN** AIGW SHALL reject the manifest without changing local configuration.

## ADDED Requirements

### Requirement: Shipped team catalogue is a curated capability contract

Team manifests SHALL declare Accounts, canonical Models, provider Routes, and
per-client Recommendations separately. Model and Route IDs SHALL be stable
lower-case; provider wire IDs and channels SHALL remain exact. Route admission
requires authenticated inference and compatible client/protocol evidence.
Setup SHALL need only one compatible Account, preserve explicit Client
Bindings, and exclude inferred Route matrices and proxy endpoints.

#### Scenario: One Account offers a subset of Models

- **WHEN** one connected Account has a compatible recommended Route but lacks
  Routes for other canonical Models or clients
- **THEN** setup SHALL activate only its usable unselected Client Bindings
- **AND** preserve explicit selections and defer unavailable capabilities.

#### Scenario: A provider uses a channel-specific wire ID

- **WHEN** DMXAPI exposes a CC, SSVIP, or CDX channel variant of one Model
- **THEN** that Route SHALL reference the same canonical Model as its base
  Route and carry its exact provider wire ID
- **AND** its lower-case Route ID SHALL not be derived from wire spelling.

#### Scenario: Ordinary Route display needs no duplicate label

- **WHEN** a Route omits `label`
- **THEN** human, JSON, and native-client presentation SHALL derive
  `Account · Model` from the declared labels
- **AND** an explicit channel or user label SHALL override that derivation
- **AND** native manifest export SHALL omit a stored label equal to the
  derived form while preserving an explicit distinct label.
