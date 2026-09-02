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
- **AND** no Proxy-specific file, process, service, port, or lifecycle state is
  required.

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
