## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, Profiles, Routes, protocol endpoints, and native
client model choices without provider identity hacks, named gateway products,
or deployment topology. Configuration manifests MUST remain credential-free
and SHALL describe available capability rather than requiring every Account to
be connected during import.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** an operator imports Accounts and Profiles for several providers
- **THEN** AIGW SHALL preserve all reviewed public capability
- **AND** SHALL allow zero or any subset of Account Tokens to be connected
- **AND** SHALL NOT require an unselected provider or absent client.

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

#### Scenario: Codex uses the selected Proxy

- **WHEN** an Account selects an implementation-neutral Responses endpoint
- **THEN** AIGW SHALL treat it as an ordinary endpoint
- **AND** Claude Code SHALL continue using its independently selected Anthropic
  endpoint.

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures an external service HTTP endpoint as an
  Account
- **THEN** AIGW SHALL treat it exactly as an external endpoint
- **AND** SHALL NOT acquire lifecycle or state ownership over that service.
