## Purpose

Define AIGW CLI as a portable provider control plane: the configuration model of
Accounts, Models, Routes and Client Bindings, provider extension and
diagnostics, and model catalog observation, with explicit authority and no
ownership of API traffic, sessions or foreign applications.

## Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, canonical Models, Routes, Client Bindings,
endpoints, and authentication without provider-specific hacks, named gateways,
topology, or global fallback. Only the current credential-free schema SHALL
execute. Offline validation SHALL serve persistence and import, report the first
failure deterministically, and derive readiness from authentication ownership.

#### Scenario: Diagnose several configuration problems

- **WHEN** the same configuration contains several invalid entries
- **THEN** repeated validation SHALL identify the same first problem
- **AND** the supplied configuration, including unset collections, SHALL remain
  unchanged.

### Requirement: Independent product authority

AIGW SHALL own only provider configuration, Account credentials, Route
selection, native Codex projection, and the native Claude Code integration. It
MUST NOT carry traffic, infer or manage an endpoint implementation, control
unrelated applications, or rewrite client-private state. Any conforming
Responses URL MAY be selected as an ordinary Route dependency and MUST be
diagnosed only through its declared protocol when that Route is activated.

#### Scenario: Team configuration selects a Responses endpoint

- **WHEN** a team manifest contains a Responses endpoint
- **THEN** manifest import SHALL remain independent of its implementation and
  lifecycle
- **AND** readiness SHALL probe it only for an installed client using that
  selected Route.

#### Scenario: Native Codex CLI and Desktop share a home

- **WHEN** AIGW discovers the native Codex Home
- **THEN** it SHALL project its marked selection into that shared `config.toml`
  without editing application-managed history or GUI state.

#### Scenario: Codex uses the selected endpoint

- **WHEN** an Account selects an implementation-neutral Responses endpoint
- **THEN** AIGW SHALL treat it as an ordinary endpoint
- **AND** Claude Code SHALL continue using its independently selected Anthropic
  endpoint.

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures an external service HTTP endpoint as an
  Account
- **THEN** AIGW SHALL treat it exactly as an external endpoint
- **AND** SHALL NOT acquire lifecycle or state ownership over that service.

### Requirement: Declarative ordinary provider extension

An ordinary provider SHALL be admitted through the provider-neutral manifest,
token-free Account, endpoint, Route, and Route data, and an optional diagnostic
registry. Adding it MUST NOT require a provider-specific command, client
projection branch, installer case, service manager, core dependency, or edits to
an existing client adapter, release path, or repository policy.

#### Scenario: A synthetic provider is imported

- **WHEN** a valid manifest adds one provider with supported protocol endpoints
  and models
- **THEN** every applicable admitted native client can select it through the
  ordinary configuration and projection path
- **AND** architecture verification proves no provider-named core branch or
  additional product owner was introduced

#### Scenario: An endpoint needs Responses compatibility

- **WHEN** an Account selects an external compatibility endpoint
- **THEN** AIGW treats it as an ordinary endpoint
- **AND** does not install, configure, start, stop, or verify that service.

### Requirement: Provider identity is not client behavior

AIGW MUST NOT encode a provider name, alias, or product identity to enable an
unrelated Codex storage, replay, authentication, or compatibility behavior.
Client projection capabilities SHALL be explicit and supported by the admitted
client contract.

#### Scenario: An endpoint needs non-default Responses behavior

- **WHEN** an Account endpoint has a distinct storage or replay expectation
- **THEN** AIGW SHALL record only an explicit supported client capability or
  endpoint choice
- **AND** it SHALL NOT rename the provider to Azure or another identity

### Requirement: Optional provider-native diagnostics

Provider-native diagnostics SHALL be optional leaf capabilities behind one
provider-neutral contract. Ordinary setup, selection, projection, check, and
endpoint verification MUST remain functional without them.

#### Scenario: No diagnostic is present

- **WHEN** a build contains no provider-native diagnostic
- **THEN** routing and native client projection SHALL continue normally
- **AND** only the explicit diagnostic surface SHALL report unavailability

### Requirement: Independent product composition

AIGW SHALL treat every valid Account endpoint as an endpoint choice, whether it
is a direct provider HTTPS endpoint or an independently operated gateway. AIGW
MUST NOT identify, import, invoke, install, configure, diagnose, reload,
uninstall, or roll back the product behind that endpoint, and the external
gateway MUST NOT acquire AIGW state.

#### Scenario: An Account selects direct HTTPS

- **WHEN** an operator selects a valid direct HTTPS Account endpoint
- **THEN** AIGW SHALL project that endpoint without requiring a local gateway
- **AND** verification SHALL exercise the selected client through that direct
  endpoint

#### Scenario: An Account selects loopback HTTP

- **WHEN** an operator selects a valid loopback endpoint
- **THEN** AIGW SHALL treat it only as an external endpoint without product,
  fixed-port, path, or lifecycle assumptions

#### Scenario: Governed Codex deployment uses an external gateway

- **WHEN** a governed deployment selects gateway endpoints for one or more
  Codex Routes
- **THEN** AIGW SHALL project those endpoints by the same Account and Route
  semantics used for direct HTTPS endpoints
- **AND** runtime evidence SHALL verify each selected client path without giving
  AIGW gateway lifecycle ownership or encoding a fixed product, path, or port

#### Scenario: An external gateway is unavailable

- **WHEN** an independently operated gateway is absent or unhealthy
- **THEN** AIGW SHALL report only the selected endpoint's observed failure
- **AND** SHALL NOT install, start, stop, repair, or reconfigure the gateway

### Requirement: Foreign applications remain independent

AIGW SHALL NOT depend on, discover, configure, align, verify, repair, or control
foreign applications or their private runtime state.

#### Scenario: A foreign application is installed

- **WHEN** AIGW runs on a machine with unrelated applications
- **THEN** product behavior and acceptance SHALL remain independent of those
  applications, their configuration, sessions, caches, and runtime

### Requirement: Token rotation is credential-scoped

AIGW SHALL validate and replace only the selected Account's Token. Rotation
MUST leave Accounts, Models, Routes, Client Bindings, client configuration, ownership
sidecars and client-owned credentials unchanged. Setup, Account connection and
rotation SHALL use the same
Token replacement and compensation owner. Compensation SHALL inspect the
written Token, preserve a different observed value, and report incomplete
storage recovery.

#### Scenario: The next helper invocation observes rotation

- **WHEN** rotation successfully replaces the selected Account's Token
- **THEN** a matching projected helper reads the replacement on its next call
- **AND** rotation invokes no client and changes no client-owned credentials
- **AND** its result does not promise immediate refresh by a running client.

#### Scenario: A newer Token appears before compensation

- **WHEN** compensation observes a Token different from the one it wrote
- **THEN** it preserves the observed Token and reports the ownership conflict
- **AND** client-owned credentials remain unchanged.

#### Scenario: The operator cancels validation

- **WHEN** the rotation command is cancelled during Token validation
- **THEN** validation observes the command cancellation
- **AND** no replacement Token is persisted.

### Requirement: Models, Routes, and Client Bindings have distinct authority

A Model SHALL identify a canonical vendor model independent of Account,
protocol, channel, recommendation, and client. A Route SHALL bind one Account
and Model to an exact upstream ID and admitted protocols. Each Client Binding
SHALL select a Route and own enabled intent, native target, protocol,
authentication, and client-only options; it alone owns selection. Each
capability requires separate proof; endpoint,
model name, catalog entry, and basic text reply do not establish it.

#### Scenario: One Route is selected by two clients

- **GIVEN** an Account exposes interfaces compatible with both clients
- **WHEN** the operator selects the same Route for both clients
- **THEN** there SHALL be one reusable Route and two independent Client Bindings
- **AND** changing one binding SHALL preserve the other client's choice and files.

#### Scenario: One Model has provider-specific route identifiers

- **GIVEN** two Accounts expose the same canonical Model under different exact
  upstream model identifiers or channel aliases
- **WHEN** the team admits both paths
- **THEN** both Routes SHALL reference the same canonical Model
- **AND** neither identifier SHALL be inferred by adding or removing a suffix.

#### Scenario: More than one endpoint is compatible

- **WHEN** a binding has multiple compatible endpoints and no explicit choice
- **THEN** selection SHALL request a protocol choice before mutation
- **AND** endpoint order, model prefixes and unrelated client settings SHALL NOT
  decide the selection.

#### Scenario: One Account exposes different protocols per model

- **GIVEN** an Account exposes Responses and Chat Completions endpoints
- **AND** two reviewed Routes on that Account were admitted on different protocols
- **WHEN** a client resolves either Route
- **THEN** only that Route's admitted protocol set SHALL be eligible
- **AND** Anthropic Messages, OpenAI Chat Completions, and OpenAI Responses
  SHALL remain distinct wire protocols
- **AND** Account-level endpoint presence SHALL NOT imply model-level compatibility.

#### Scenario: Responses text and Responses reasoning differ

- **GIVEN** a Route can complete a basic OpenAI Responses text request
- **WHEN** its reasoning, tool, continuation, or compaction behavior has not been
  qualified for the selected client
- **THEN** AIGW SHALL report only the proven text capability
- **AND** reasoning, streaming, tool calls, structured output, continuation,
  compaction, and multimodal input SHALL be qualified separately
- **AND** SHALL NOT present the Route as Codex-compatible or reasoning-qualified.

#### Scenario: The team recommends a Route

- **WHEN** the reviewed team catalog recommends a primary Route and bounded
  alternatives for a client context
- **THEN** those positions SHALL be relational guidance rather than Model tiers
- **AND** an explicit Client Binding SHALL remain unchanged by import, discovery,
  or a later recommendation.

#### Scenario: The operator disables a client

- **WHEN** a client is disabled and its owned projection is withdrawn
- **THEN** synchronization, discovery and team import SHALL retain disabled intent
- **AND** the reusable Route, credential reference and other clients remain intact.

#### Scenario: A supported but unused client is discovered

- **WHEN** the operator inspects an otherwise healthy configured installation
- **THEN** an unselected client SHALL NOT create a health failure or mandatory
  setup action
- **AND** optional availability SHALL remain discoverable separately.

### Requirement: Schema replacement is explicit and reversible

AIGW SHALL migrate the prior supported configuration through a bounded explicit
operation with a preview of Accounts, canonical Models, equivalent Routes,
Client Bindings and native options. Normal runtime SHALL interpret only the
current schema. The
migration SHALL preserve explicit choices, disabled intent, credentials in their
authoritative stores, unrelated client settings and existing session metadata.

#### Scenario: The migration preview has not been applied

- **WHEN** the operator previews the prior configuration's canonical replacement
- **THEN** no configuration, credential, client projection or session SHALL change
- **AND** ambiguous identity or ownership SHALL be reported before application.

#### Scenario: State changes after migration preparation

- **WHEN** any affected preimage differs before its prepared write
- **THEN** migration SHALL preserve that state and report the conflict
- **AND** already applied owned writes SHALL use guarded compensation.

#### Scenario: The new schema is accepted

- **WHEN** retained-state migration and native candidate acceptance pass
- **THEN** old normal-runtime readers and duplicate selection state SHALL be removed
- **AND** rollback SHALL use the immutable predecessor and matching retained state,
  not a second live schema authority.

### Requirement: Model admission follows protocol and capability

AIGW SHALL admit model identifiers independently of client branding, using the
selected endpoint's protocol and verified capabilities. Provider-native APIs
SHALL be preferred when sufficient. An independently selected compatibility
service MAY supply a missing protocol without becoming part of AIGW's runtime.
Displayed mappings SHALL preserve the identity of the requested and serving
models. A successful text request SHALL establish only text connectivity.

#### Scenario: A non-Anthropic model serves Claude

- **WHEN** an explicit Client Binding selects a non-Anthropic model through a compatible
  Messages endpoint
- **THEN** AIGW SHALL preserve that provider and model identity in its output
- **AND** support claims SHALL reflect tested streaming, tool results,
  continuation, and model-specific limits.

#### Scenario: Codex uses an independent Responses endpoint

- **WHEN** an explicit Client Binding selects a model through an independent Responses
  endpoint
- **THEN** validation SHALL verify its actual input, event, and tool contracts
- **AND** missing compaction or hosted tools SHALL have explicit outcomes
- **AND** existing conversation metadata and native model selections remain unchanged.

#### Scenario: A provider exposes a smaller capability set

- **WHEN** an endpoint does not support a capability required by a client journey
- **THEN** the journey SHALL report the specific unsupported capability
- **AND** configuration or a renamed model SHALL NOT be presented as full acceptance.

### Requirement: Diagnostic quota and throttling remain distinct

An authenticated diagnostic SHALL use explicit provider error evidence, not HTTP
status alone, to distinguish exhausted Account credit or quota from transient
rate or concurrency limits. Classification SHALL NOT switch Routes or retry a
request automatically.

#### Scenario: A provider reports hard quota exhaustion with HTTP 429

- **WHEN** an Account diagnostic receives HTTP 429 with an unambiguous exhausted
  quota code or insufficient balance evidence
- **THEN** AIGW SHALL report non-retryable Account quota exhaustion and give
  Account-scoped recovery guidance

#### Scenario: A provider reports transient throttling with HTTP 429

- **WHEN** an Account diagnostic receives HTTP 429 with rate-limit or ambiguous
  concurrency-quota evidence
- **THEN** AIGW SHALL report retryable throttling rather than exhausted credit
- **AND** the diagnostic SHALL have made only its single admitted request.

### Requirement: Route and Client Binding compatibility

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

#### Scenario: Import a multi-provider team catalog

- **WHEN** a team manifest declares recommended Routes for admitted clients
- **THEN** setup SHALL retain those recommendations separately from actual
  per-client selections and fill only currently usable unselected Routes
- **AND** neither a global default nor an unavailable recommendation SHALL
  replace an explicit client selection.
- **AND** a future client SHALL remain unselected until its own Route is
  explicitly admitted.

#### Scenario: Diagnose a partially connected team catalog

- **WHEN** a reviewed catalog contains multiple Accounts
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

### Requirement: Model discovery reports catalog evidence only

`aigw catalog` and `aigw models` SHALL share one Account catalog observation
path. Route comparison SHALL distinguish listed, not listed, and an unobserved
catalog with its reason. These commands MUST NOT describe catalog membership
as inference reachability or native-client readiness. An incomplete, oversized
or ambiguous HTTP response SHALL produce a failed catalog observation, not
partial success.

#### Scenario: A configured model appears in the catalog

- **WHEN** an authenticated catalog response lists the configured model ID
- **THEN** `models` reports that ID as listed
- **AND** performs no inference or client invocation.

#### Scenario: A model is absent from a successfully observed catalog

- **WHEN** a complete catalog does not list the configured model ID
- **THEN** `models` reports not listed rather than unavailable.

#### Scenario: Catalog observation fails

- **WHEN** credentials cannot be observed, the endpoint is absent, the request
  fails, or the response is incomplete or oversized
- **THEN** the affected Account has no established model membership
- **AND** its observation reason remains visible without blocking other Accounts.

#### Scenario: A catalog response is ambiguous

- **WHEN** a successful HTTP response has a null model list, repeats that list,
  or repeats an identifier member within one item
- **THEN** the affected Account has no established model membership, and
  `models` SHALL NOT report its Routes as not listed
- **AND** an empty list, the supported identifier shapes and other provider
  fields SHALL still produce a successful observation.

### Requirement: Catalog observation owns an isolated and bounded probe lifetime

Bundled and effective catalog readers SHALL execute with an operation-owned
client home and bounded subprocess lifetime. They SHALL preserve the operator's
configuration and report cleanup failures with the exact owned path and original
failure causes. Repository verification SHALL apply the same ownership rule to
its generated catalog input.

#### Scenario: Client catalog retrieval fails after identifying the executable

- **WHEN** the client reports its version but catalog retrieval fails
- **THEN** the bundled reader SHALL retain the measured executable identity and
  original command failure
- **AND** successful cleanup SHALL remove the isolated home; failed cleanup
  SHALL add its cause without discarding the command failure or measured identity.

### Requirement: Provider catalog evolution is observed before admission

Catalogs are observations, not authority. AIGW SHALL bound refreshes
and diff normalized exact upstream IDs against admitted Models and Routes
deterministically. Evidence stays derived outside the reviewed manifest.
Admission SHALL track observed, candidate, qualified, admitted, deprecated,
and retired. New entries SHALL NOT change configuration,
recommendations, bindings, or projections before reviewed admission; missing
entries SHALL NOT prove withdrawal, rename, or incompatibility.

#### Scenario: A provider publishes a new model

- **WHEN** catalog refresh observes a previously unknown upstream identifier
- **THEN** AIGW SHALL report a deterministic candidate Model or Route difference
- **AND** SHALL require real protocol and client qualification before admission
- **AND** SHALL require review before changing a recommendation.

#### Scenario: A provider catalog omits an admitted Route

- **WHEN** one refresh no longer reports an admitted upstream identifier
- **THEN** AIGW SHALL retain the Route and explicit Client Bindings
- **AND** SHALL report stale or contradictory evidence for bounded requalification
- **AND** SHALL NOT infer a rename from another newly observed identifier.

#### Scenario: An admitted primary Route becomes unusable

- **WHEN** current qualification proves that a recommended primary Route no
  longer satisfies the selected client's contract
- **THEN** AIGW MAY propose a previously admitted alternative
- **AND** SHALL NOT change an explicit Client Binding without operator intent.

### Requirement: Stored capabilities follow their recorded Route

AIGW SHALL accept a capability outside the published schema vocabulary at its
earlier location in the canonical configuration and SHALL move it to the
separate component at its next configuration write. A stored capability SHALL
apply only while its Route keeps the Account, endpoint, upstream model and
interface it was recorded for.

#### Scenario: An earlier release wrote the capability into the canonical configuration

- **WHEN** the canonical configuration declares a capability outside the
  published schema vocabulary
- **THEN** AIGW SHALL accept the configuration with that capability declared
- **AND** its next configuration write SHALL remove the capability from the
  canonical configuration without changing what the Route declares.

#### Scenario: The recorded Route changes

- **WHEN** the Route of a stored capability no longer exists or no longer
  matches the Account, endpoint, upstream model or interface it was recorded
  for
- **THEN** the capability SHALL NOT apply
- **AND** the next configuration write SHALL drop it.
