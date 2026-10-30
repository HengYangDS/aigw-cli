## Purpose

Define how an admitted client's adapter joins AIGW: its admission, the extension
boundary that every adapter shares, the client surfaces it serves, its native
provider declarations and how the control plane converges each client.

## ADDED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independent Adapters owning discovery,
projection, authentication, rollback, verification, status, and withdrawal of
AIGW state. Account-Token credential commands MUST use an absolute qualified
reader and Route, never store Tokens in client files, and never assume a
package-manager CLI link stays available. Client-native authentication and
preferences SHALL remain client-owned. A new client MUST add an Adapter without
changing provider policy or other Adapters.

#### Scenario: Verification uses synchronized settings

- **WHEN** AIGW verifies an admitted client
- **THEN** it SHALL use the client's synchronized settings without a wrapper.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the selected qualified reader
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** an enabled Codex Route selects an explicit native provider identity
- **THEN** its declared authentication mode SHALL determine credential ownership
- **AND** Account-Token authentication SHALL invoke the selected qualified
  reader for only the active Codex Route
- **AND** client-native authentication SHALL project no AIGW Token helper
- **AND** neither mode SHALL write a plaintext Token to public configuration

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** explicit verification invokes Claude Code for an admitted Route
- **THEN** the native client SHALL consume its synchronized settings in bare,
  nonpersistent mode
- **AND** stale AIGW-owned Anthropic environment overrides SHALL be removed
- **AND** unrelated client preferences, including beta controls, SHALL remain
  unchanged rather than becoming a new AIGW authority

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: The credential reader path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the selected reader path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled adapter,
  an unresolved Route, or an Account without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, external-gateway behavior,
  command roots, or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected Route once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

#### Scenario: An adapter is proposed for another agent client

- **WHEN** a new agent client such as pi or OpenCode is proposed
- **THEN** its admission SHALL independently prove executable discovery,
  protocol/model selection, credential ownership, native configuration merge,
  rollback, withdrawal and a real tool-loop journey
- **AND** AIGW SHALL NOT advertise support based only on a writable config
  path or a successful endpoint probe.

### Requirement: Route-scoped Codex native provider

A Route SHALL remain the sole owner of its client, Account, model, and
optional client-native provider selection. A missing Codex provider selection
SHALL resolve to the canonical `aigw` provider. Provider selection SHALL NOT be
inferred from an Account, endpoint, proxy implementation, or client-private
state. Authentication ownership SHALL be independent of the provider name.

#### Scenario: Explicit Codex provider

- **WHEN** a Codex-scoped Route declares a safe `model_provider`
- **THEN** its resolved Runtime carries that exact provider identity
- **AND** Codex receives one attributed provider table using the Route's
  Account endpoint and declared authentication mode.

#### Scenario: Default Codex provider

- **WHEN** a Codex-scoped Route omits `model_provider`
- **THEN** its Runtime resolves the canonical `aigw` provider
- **AND** Account-Token authentication uses the same command-helper contract
  as an explicit provider.

#### Scenario: Provider ownership is narrow

- **WHEN** a non-Codex Route or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection.

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic.
Endpoints and models SHALL enter as Account data; existing
client-native authentication SHALL be reused through explicit Routes. New
clients SHALL require complete Adapters, unsupported credential contracts SHALL
require admission, and incompatible wire behavior SHALL remain in a separate
data plane. Gateways SHALL remain optional endpoints. Dependencies MAY be
admitted only when total owned complexity falls.

#### Scenario: Add a compatible Provider endpoint

- **WHEN** an endpoint satisfies an admitted client protocol and existing
  Account authentication contract
- **THEN** an operator SHALL add it through configuration data
- **AND** AIGW SHALL NOT add provider-name branching or a provider-specific
  runtime package.

#### Scenario: Add a client integration

- **WHEN** a new local client requires AIGW-managed configuration
- **THEN** it SHALL be admitted through its own discovery, planning, guarded
  projection, verification, rollback, and uninstall boundary
- **AND** it SHALL NOT reuse another client's conditional path or private state.

#### Scenario: A client adapter is registered

- **WHEN** an adapter is registered, including a test-only rehearsal adapter
- **THEN** it SHALL pass one shared adapter contract: its plan lists every
  file that applying it may change and states exactly whether applying it
  changes one, rollback restores the prior bytes and modes, withdrawal leaves
  no owned file, and inspection, verification and planning change no file and
  leave the caller's configuration unchanged
- **AND** an adapter that fails any part of that contract SHALL NOT be admitted.

#### Scenario: Compose with a traffic gateway

- **WHEN** an operator selects a general gateway or narrow compatibility
  service
- **THEN** AIGW SHALL model its URL as an ordinary Account endpoint
- **AND** SHALL NOT install, supervise, configure, embed, or copy the service's
  traffic policy.

#### Scenario: Evaluate a mature dependency

- **WHEN** a library or framework is proposed for an AIGW-owned boundary
- **THEN** admission SHALL demonstrate a net reduction in owned complexity
- **AND** popularity or feature count alone SHALL NOT justify adoption.

### Requirement: Control-plane convergence is client-scoped and monotonic

AIGW SHALL derive client state from its explicit Client Binding and referenced
Account, canonical Model, exact Route, Adapter, and credential backend. Only the
binding owns selection and enabled intent. Setup, selection, sync, and readiness
SHALL require no global selection, other binding, or external compatibility
product. Discovery SHALL not create native state or enable an unbound client.
Projection SHALL preserve unrelated fields and reject changed preimages.

#### Scenario: A client is discovered but not configured

- **WHEN** discovery finds an admitted executable whose native state has not
  been initialized or explicitly bound
- **THEN** AIGW SHALL report it as available without creating its configuration
- **AND** synchronization and readiness of other clients remain unaffected.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one Codex Route and one Claude Route in
  separate operations
- **THEN** both per-client bindings remain selected
- **AND** readiness requires no additional aggregate selection operation.

#### Scenario: An ordinary configuration edit affects one client

- **WHEN** an Account, Model, or Route edit changes one client's persistent projection
- **THEN** the configuration transaction SHALL plan and apply only the affected
  client set in admission order
- **AND** another client's external edits and ownership state SHALL remain
  byte-identical, without being inspected as a prerequisite for that edit.

#### Scenario: A shared edit affects multiple clients

- **WHEN** an edit changes several client projections
- **THEN** all affected clients SHALL participate in the same guarded
  transaction and existing reverse compensation contract
- **AND** explicit synchronization or repair SHALL still reconcile its entire
  requested client scope, including unchanged configuration.

#### Scenario: One client is absent

- **WHEN** a valid selected Client Binding belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent binding remains usable.

#### Scenario: An external compatibility endpoint is absent

- **WHEN** no local compatibility service is installed
- **THEN** AIGW remains usable with any configured native HTTPS endpoint
- **AND** no external-gateway file, process, service, port, or lifecycle state
  is required.

#### Scenario: Cancellation precedes mutation admission

- **WHEN** setup, configuration selection, projection, or repair observes a
  cancelled request at its mutation boundary
- **THEN** it returns cancellation without starting that boundary's writes
- **AND** setup restores any Token already prepared before cancellation was
  observed, subject to the existing credential ownership guard.

#### Scenario: Preparation observes cancellation before persistence

- **WHEN** snapshot capture or projection preflight completes with the request
  cancelled
- **THEN** configuration persistence does not start
- **AND** existing configuration, backup, checkpoint and client files remain
  unchanged.

#### Scenario: Cancellation prevents a later client projection

- **WHEN** cancellation is observed before the next client adapter starts
- **THEN** that adapter performs no writes
- **AND** the registry compensates prior adapters in reverse order and the
  enclosing transaction compensates its configuration
- **AND** cancellation and any recovery conflicts remain inspectable errors.

#### Scenario: Cancellation follows the last admitted projection

- **WHEN** the last admitted adapter completes successfully after cancellation
  arrives during its synchronous operation
- **THEN** the completed transaction remains successful
- **AND** no post-completion cancellation check undoes its committed result.

#### Scenario: Identity migration is cancelled before writes

- **WHEN** Account renaming or verified finalization observes cancellation
  before credential preparation or finalization admission
- **THEN** configuration, backup and source and target credentials remain
  unchanged
- **AND** cancellation returns from the identity-migration owner independently
  of the CLI; credential copies already admitted before a later configuration
  commit failure retain both slots for retry and rollback.

#### Scenario: Setup is requested for an existing installation

- **GIVEN** the current configuration already contains Routes
- **WHEN** interactive, explicit, manifest or internal setup is requested
- **THEN** the shared setup admission rejects it before prompting, Provider
  probing, client discovery or mutation
- **AND** configuration import, selection and Token rotation retain their
  separate operations rather than becoming implicit setup behavior.

#### Scenario: First-time setup finds an existing credential slot

- **GIVEN** the current configuration has no Routes but an Account Token is
  already available
- **WHEN** first-time setup is requested
- **THEN** that Token does not by itself classify the installation as configured
- **AND** setup may use or explicitly replace it with guarded compensation.

### Requirement: Extension preserves the control-plane core

An ordinary Provider SHALL be added through Account, endpoint, Route, Route,
and optional diagnostic declarations. A new client SHALL be added through one
admitted client Adapter and its conformance fixtures. Neither extension SHALL
introduce Provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add a client-native cloud model service

- **WHEN** an AWS model service exposes an admitted client protocol and the
  client already owns its credential chain and request signing
- **THEN** its Account and Routes use the ordinary manifest path
- **AND** the Route declares client-native authentication
- **AND** AIGW neither stores the cloud credential nor implements a signing
  Adapter.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing Provider data and client Adapters remain unchanged.

#### Scenario: Supply operational adapters in a different order

- **WHEN** the registry receives operational adapters in an order different
  from its admitted client declarations
- **THEN** discovery, planning and all-client application SHALL follow admission
  order, independently of constructor argument order
- **AND** a failed application SHALL compensate completed adapters in reverse
  application order.

#### Scenario: An adapter's compensation fails

- **WHEN** application fails after prior adapters have completed and one prior
  adapter reports a compensation failure
- **THEN** the registry SHALL still attempt every other prior adapter's
  compensation in reverse application order
- **AND** the returned error SHALL preserve the application failure and each
  compensation failure without claiming successful restoration.

### Requirement: Requested client surfaces have explicit adapters

AIGW SHALL integrate Hermes and Claude Desktop through their supported native
configuration and credential interfaces. Each Adapter SHALL use the shared
Client Binding, transaction, discovery, verification, and withdrawal owners. Claude
Desktop Chat, Cowork, and Code SHALL retain distinct capability evidence.
Client-owned sessions, services, and model choices SHALL remain client-owned.

#### Scenario: Hermes becomes available after team import

- **GIVEN** team configuration was imported before Hermes was installed
- **WHEN** Hermes becomes available and its Route is explicitly selected
- **THEN** synchronization SHALL configure its admitted provider and model
- **AND** credentials SHALL use the selected supported delivery mechanism
- **AND** unrelated clients, Hermes sessions, and Hermes services remain unchanged.

#### Scenario: Hermes exposes the connected catalog

- **GIVEN** more than one Account has an available credential
- **WHEN** Hermes synchronization projects its native model catalog
- **THEN** every admitted Route compatible with Hermes from those connected
  Accounts SHALL be grouped under its Account and protocol provider
- **AND** the explicitly selected Route SHALL remain Hermes' active model
- **AND** disconnected Accounts and unqualified Routes SHALL remain absent.

#### Scenario: Claude Desktop and Claude Code use different bindings

- **WHEN** the operator enables distinct Desktop and CLI Routes
- **THEN** each surface SHALL use its own documented configuration boundary
- **AND** status SHALL state which Desktop modes and platforms were qualified
- **AND** a required app restart SHALL be reported before activation is claimed.

### Requirement: Claude Desktop tool search follows Route qualification

AIGW SHALL enable Claude Desktop host tool search in its owned gateway profile
only when every Route the profile exposes declares the `tool_search`
capability on its Anthropic interface. That capability records a native
Desktop session's qualification of the tool-search beta, deferred tool loading
and `tool_reference` results on that interface. Configuration validation SHALL
reject `tool_search` on any other interface.

#### Scenario: Every exposed Route qualified tool search

- **WHEN** the selected Route and every other Route in the Desktop profile
  declare `tool_search` on their Anthropic interface
- **THEN** synchronization SHALL write the profile with tool search enabled
- **AND** inspection SHALL report that projection unchanged.

#### Scenario: One exposed Route lacks the qualification

- **WHEN** any Route in the Desktop profile does not declare `tool_search`
- **THEN** the profile SHALL omit the tool-search setting
- **AND** a tool-search setting added outside AIGW SHALL fail closed as
  managed drift.

#### Scenario: An exposed Route's qualification changes

- **WHEN** an exposed Route gains or loses `tool_search`
- **THEN** synchronization SHALL report a projection change and rewrite the
  profile to match.

#### Scenario: A non-Anthropic interface declares tool search

- **WHEN** a Route declares `tool_search` on an OpenAI interface
- **THEN** configuration validation SHALL reject that Route.
