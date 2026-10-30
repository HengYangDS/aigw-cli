## ADDED Requirements

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

## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Route and binding compatibility

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Route and Client Binding compatibility" in this capability, with every scenario.

### Requirement: Transactional and inspectable projection

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: Portable source

**Reason**: The `repository-organization` capability owns this requirement's subject.

**Migration**: Read it in `repository-organization`, with every scenario.

### Requirement: Complete Forge commit provenance

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Enforced semantic ownership and quality

**Reason**: The `repository-organization` capability owns this requirement's subject.

**Migration**: Read it in `repository-organization`, with every scenario.

### Requirement: Deterministic local verification

**Reason**: The `repository-organization` capability owns this requirement's subject.

**Migration**: Read it in `repository-organization`, with every scenario.

### Requirement: Source-bound quantitative evidence

**Reason**: The `repository-organization` capability owns this requirement's subject.

**Migration**: Read it in `repository-organization`, with every scenario.

### Requirement: Quiet handled failures

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Recoverable published-history repair

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Atomic published-history replacement

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Independently admitted native clients

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Hosted evidence identity is Forge-portable

**Reason**: The `ci-diagnostics` capability owns this requirement's subject.

**Migration**: Read it in `ci-diagnostics`, with every scenario.

### Requirement: Native client fixtures are repository-controlled

**Reason**: The `repository-organization` capability owns this requirement's subject.

**Migration**: Read it in `repository-organization`, with every scenario.

### Requirement: Local-first independent publication topology

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Latest stable repository-owned supply chain

**Reason**: The `supply-chain` capability owns this requirement's subject.

**Migration**: Read it in `supply-chain`, with every scenario.

### Requirement: Terminal candidate integration is exact and local

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Terminal local release readiness

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Reviewed team configuration is directly consumable

**Reason**: The `progressive-team-onboarding` capability owns this requirement's subject.

**Migration**: Read it in `progressive-team-onboarding`, with every scenario.

### Requirement: Route-scoped Codex native provider

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Native released-artifact lifecycle acceptance

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Composable extension boundary

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Real Codex client route verification

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Windows path derivation preserves the selected namespace

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Model discovery reports catalogue evidence only

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Model discovery reports catalog evidence only" in this capability, with every scenario.

### Requirement: Control-plane convergence is client-scoped and monotonic

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Extension preserves the control-plane core

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Client deactivation is ownership-bounded

**Reason**: The `route-client-selection` capability owns this requirement's subject.

**Migration**: Read it in `route-client-selection`, with every scenario.

### Requirement: Portable installation describes its own current files

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Verification checkpoints remain bound to current configuration

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Release helpers preserve selected source identity

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Local candidate identity is verified before a no-op result

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Repeated portable installation preserves rollback identity

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Projection-matched Account Token delivery

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: Readiness observations remain separated from client execution

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Recovery execution and presentation are independent

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Invocation output has one owner

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Command help preserves native metadata

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Argument admission precedes mutation

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Program replacement has an explicit client reconciliation boundary

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Online update owns its temporary resources and preserves failure causes

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Release construction owns only its operation resources

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Peer ancestry observation preserves local reference state

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Catalogue observation owns an isolated and bounded probe lifetime

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Catalog observation owns an isolated and bounded probe lifetime" in this capability, with every scenario.

### Requirement: Failure recovery retains owned-resource cleanup outcomes

**Reason**: The `program-lifecycle` capability owns this requirement's subject.

**Migration**: Read it in `program-lifecycle`, with every scenario.

### Requirement: Provider catalogue evolution is observed before admission

**Reason**: Its title or scenario names spell catalog as the glossary does, and OpenSpec renames a scenario only with its requirement.

**Migration**: Read it as "Provider catalog evolution is observed before admission" in this capability, with every scenario.

### Requirement: Requested client surfaces have explicit adapters

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Account connection is an atomic client-scoped transition

**Reason**: The `progressive-team-onboarding` capability owns this requirement's subject.

**Migration**: Read it in `progressive-team-onboarding`, with every scenario.

### Requirement: Shipped team catalogue is a curated capability contract

**Reason**: The `progressive-team-onboarding` capability owns this requirement's subject.

**Migration**: Read it in `progressive-team-onboarding`, titled "Shipped team catalog is a curated capability contract", with every scenario.

### Requirement: Hermes verification excludes unrelated update services

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Pre-tag artifact acceptance binds to signed source

**Reason**: The `release-distribution` capability owns this requirement's subject.

**Migration**: Read it in `release-distribution`, with every scenario.

### Requirement: Hermes verification reuses installed dependency state

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Synchronization adopts a Hermes model choice

**Reason**: The `route-client-selection` capability owns this requirement's subject.

**Migration**: Read it in `route-client-selection`, with every scenario.

### Requirement: Client projection failure reports verified recovery state

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: The shipped catalogue keeps one general Model per other vendor

**Reason**: The `progressive-team-onboarding` capability owns this requirement's subject.

**Migration**: Read it in `progressive-team-onboarding`, titled "The shipped catalog keeps one general Model per other vendor", with every scenario.

### Requirement: Claude Desktop tool search follows Route qualification

**Reason**: The `client-adapters` capability owns this requirement's subject.

**Migration**: Read it in `client-adapters`, with every scenario.

### Requirement: Synchronization adopts a Codex model choice

**Reason**: The `route-client-selection` capability owns this requirement's subject.

**Migration**: Read it in `route-client-selection`, with every scenario.

### Requirement: Local failures name the failed stage

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Claude Desktop keeps its own profile settings

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: Repair previews name each projection surface

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: Hermes verification releases its gateway before deleting its home

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Hermes verification waits for its gateway process to exit

**Reason**: The `cli-readiness` capability owns this requirement's subject.

**Migration**: Read it in `cli-readiness`, with every scenario.

### Requirement: Route presentation groups admitted service types

**Reason**: The `command-line` capability owns this requirement's subject.

**Migration**: Read it in `command-line`, with every scenario.

### Requirement: Native verification runs the declared Mise release

**Reason**: The `supply-chain` capability owns this requirement's subject.

**Migration**: Read it in `supply-chain`, with every scenario.

### Requirement: Real-client acceptance supplies each host's official clients

**Reason**: The `supply-chain` capability owns this requirement's subject.

**Migration**: Read it in `supply-chain`, with every scenario.

### Requirement: An outside edit to managed Codex settings has one recovery action

**Reason**: The `projection-format` capability owns this requirement's subject.

**Migration**: Read it in `projection-format`, with every scenario.

### Requirement: A vulnerable exact transitive pin is overridden

**Reason**: The `supply-chain` capability owns this requirement's subject.

**Migration**: Read it in `supply-chain`, with every scenario.
