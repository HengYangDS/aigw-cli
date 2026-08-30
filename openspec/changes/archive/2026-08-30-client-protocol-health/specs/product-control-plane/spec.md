## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, client-scoped Profiles, explicit client Routes,
protocol endpoints, and native client model choices without provider identity
hacks, named gateway products, deployment topology, or a global Profile
fallback. A Route SHALL map one admitted client to one compatible Profile.
Configuration manifests MUST remain credential-free and SHALL describe
available capability rather than requiring every Account to be connected during
import. Diagnostics SHALL require a credential only for an Account selected by
an enabled admitted-client Route. `aigw check` SHALL NOT claim overall health
unless every enabled admitted-client Route has its selected Account Token and
its distinct authentication target has passed the bounded health probe.

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

- **WHEN** a previous schema contains client overrides and a global default
- **THEN** every override SHALL migrate directly to the same client Route
- **AND** an unclaimed default SHALL migrate only to the client explicitly
  declared by that Profile
- **AND** an ambiguous default SHALL require explicit operator selection rather
  than being guessed or retained as a compatibility fallback.

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
