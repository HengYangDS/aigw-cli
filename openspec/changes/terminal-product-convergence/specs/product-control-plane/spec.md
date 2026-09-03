## ADDED Requirements

### Requirement: Control-plane convergence is client-scoped and monotonic

AIGW SHALL derive operational state only from Accounts, client-scoped Profiles,
explicit per-client Routes, admitted client Adapters, and the selected
credential backend. Setup, selection, synchronization, and readiness MUST NOT
depend on a global Profile, an aggregate selection flag, another client's
Route, or the presence of an external compatibility product.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one Codex Profile and one Claude Profile in
  separate operations
- **THEN** both per-client Routes remain selected
- **AND** readiness requires no additional aggregate selection operation.

#### Scenario: One client is absent

- **WHEN** a valid selected Route belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent Route remains usable.

#### Scenario: An external compatibility endpoint is absent

- **WHEN** no local compatibility service is installed
- **THEN** AIGW remains usable with any configured native HTTPS endpoint
- **AND** no external-gateway file, process, service, port, or lifecycle state
  is required.

### Requirement: Extension preserves the control-plane core

An ordinary Provider SHALL be added through Account, endpoint, Profile, Route,
and optional diagnostic declarations. A new client SHALL be added through one
admitted client Adapter and its conformance fixtures. Neither extension SHALL
introduce Provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add an AWS model service

- **WHEN** an AWS model service exposes an admitted client protocol
- **THEN** its Account and Profiles use the ordinary manifest path
- **AND** only an irreducibly Provider-specific diagnostic or signing adapter
  requires code.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing Provider data and client Adapters remain unchanged.

### Requirement: Client deactivation is ownership-bounded

AIGW SHALL withdraw client integration through the same guarded projection
transaction that created it. Disabling one Adapter SHALL remove only that
client's AIGW-owned configuration block, sidecar, generated catalogue, and
credential-helper or command projection. Portable uninstall SHALL first disable
every enabled Adapter and SHALL remove the executable only after client
withdrawal succeeds. Both operations SHALL preserve Accounts, Profiles, Routes,
Tokens, other enabled Adapters, and neighboring user-authored client state.

A successful disable or uninstall SHALL remove the verified checkpoint because
it describes client projections that are no longer present. The single previous
configuration backup MAY remain as an explicit operator-selected rollback
source; it MUST NOT be treated as current state or applied implicitly.

#### Scenario: Disable one client

- **WHEN** an operator disables one enabled client Adapter
- **THEN** only that client's AIGW-owned projection and ownership state are
  withdrawn
- **AND** the other client, capability configuration, credentials, and
  user-authored client settings remain unchanged
- **AND** the stale verified checkpoint is absent.

#### Scenario: Uninstall the portable program

- **WHEN** an operator uninstalls AIGW with one or more enabled client Adapters
- **THEN** every AIGW-owned client projection is withdrawn before the executable
  and its program rollback copy are removed
- **AND** capability configuration, credentials, neighboring user state, and the
  explicit previous-configuration backup remain available
- **AND** no verified checkpoint continues to claim the withdrawn projections.

## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, client-scoped Profiles, explicit client Routes,
protocol endpoints, and native client model choices without provider identity
hacks, named gateway products, deployment topology, or a global Profile
fallback. A Route SHALL map one admitted client to one compatible Profile.
The current configuration schema SHALL be the only executable local schema;
earlier schemas require an explicit operator-led replacement rather than an
embedded migration path. Configuration manifests MUST remain credential-free
and SHALL describe available capability rather than requiring every Account to
be connected during import. Diagnostics SHALL require a credential only for an
Account selected by an enabled admitted-client Route. `aigw check` SHALL NOT
claim overall health unless every enabled admitted-client Route has its selected
Account Token and its distinct authentication target has passed the bounded
health probe.

#### Scenario: Select independent Claude and Codex services

- **WHEN** an operator selects a Claude Profile and a Codex Profile
- **THEN** each client SHALL retain its own explicit Route
- **AND** no global default or implicit inheritance SHALL participate in
  resolution, readiness, or projection.

#### Scenario: Select a Profile without repeating its client

- **WHEN** an operator runs `aigw use <profile>`
- **THEN** AIGW SHALL derive the target client from the Profile's declared
  client
- **AND** SHALL reject a Profile whose client is absent or unadmitted.

#### Scenario: Check every enabled client Route

- **WHEN** Claude and Codex are enabled with distinct selected Routes
- **THEN** `aigw check` SHALL validate both effective Routes
- **AND** SHALL derive each authentication request from that Route's declared
  client protocol
- **AND** SHALL not inspect an unselected historical Profile as a fallback
- **AND** MAY coalesce only authentication probes with an identical Account,
  endpoint, and protocol identity.

#### Scenario: No client is enabled

- **WHEN** configuration is valid but no admitted client Adapter is enabled
- **THEN** `aigw check` SHALL report configuration readiness
- **AND** SHALL not claim that an arbitrary gateway or model is healthy.

#### Scenario: Read previous local configuration

- **WHEN** AIGW reads its local configuration
- **THEN** it SHALL accept the current schema with explicit per-client Routes
- **AND** an earlier schema SHALL return its actual version and the current
  required version without inferring or migrating Route selection.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** a team manifest declares recommended Routes for admitted clients
- **THEN** setup SHALL materialize those per-client selections without a
  separate recommended global default
- **AND** a future client SHALL remain unselected until its own Route is
  explicitly admitted.

#### Scenario: Diagnose a partially connected team catalogue

- **WHEN** a reviewed catalogue contains multiple Accounts
- **AND** every Account selected by an enabled client Route has its Token
- **THEN** `aigw doctor` SHALL report the credential state as healthy
- **AND** SHALL NOT fail for an unselected Account whose Token is absent.

#### Scenario: Diagnose a selected Account without a Token

- **WHEN** an enabled client Route selects an Account whose Token is absent
- **THEN** `aigw doctor` SHALL report that Account as unhealthy
- **AND** SHALL provide the account-scoped rotation action.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Profile data for a new
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
