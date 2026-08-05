## ADDED Requirements

### Requirement: Declarative ordinary provider extension

An ordinary provider SHALL be admitted through token-free Account, endpoint,
Profile, and Route data. Adding it MUST NOT require a provider-specific command,
client projection branch, installer case, service manager, or core dependency.

#### Scenario: A synthetic provider is imported

- **WHEN** a valid manifest adds one provider with supported protocol endpoints
  and models
- **THEN** every applicable admitted native client can select it through the
  ordinary configuration and projection path
- **AND** architecture verification proves no provider-named core branch or
  additional product owner was introduced

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

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be the admitted native clients. Each adapter SHALL
own discovery, supported configuration or process planning, authentication,
rollback, verification, status, and uninstall of only its AIGW-owned state.
Adding a future client MUST NOT change provider policy or another adapter.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, Proxy behavior, command roots,
  or an existing adapter

### Requirement: Independent product composition

AIGW MAY compose with any valid Account endpoint, including Codex Responses
Proxy. It MUST NOT import, invoke, install, configure, diagnose, reload,
uninstall, or roll back the Proxy, and the Proxy MUST NOT acquire AIGW state.

#### Scenario: An Account selects loopback HTTP

- **WHEN** an operator selects a valid loopback endpoint
- **THEN** AIGW SHALL treat it only as an external endpoint without product,
  fixed-port, path, or lifecycle assumptions

#### Scenario: Governed Codex deployment uses the Proxy

- **WHEN** this closeout accepts UCloud, DMXAPI, and AIHubMix Codex routes
- **THEN** native Codex traffic SHALL use the AIGW-projected Codex Responses
  Proxy endpoint for each route
- **AND** runtime evidence SHALL prove the Proxy-to-provider path without
  giving AIGW Proxy lifecycle ownership or encoding a fixed path or port

### Requirement: Foreign applications remain independent

AIGW and Proxy SHALL NOT depend on, discover, configure, align, verify, repair,
or control foreign applications or their private runtime state.

#### Scenario: A foreign application is installed

- **WHEN** AIGW or Proxy runs on a machine with unrelated applications
- **THEN** product behavior and acceptance SHALL remain independent of those
  applications, their configuration, sessions, caches, and runtime

## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, Profiles, Routes, protocol endpoints, and native
client model choices without provider identity hacks, named gateway products,
or deployment topology. Configuration manifests MUST remain credential-free.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Profile data for a new
  endpoint
- **THEN** Codex or Claude Code SHALL select it without a provider-specific CLI,
  installer, projection branch, service manager, or core dependency

#### Scenario: A Responses endpoint needs compatibility behavior

- **WHEN** an endpoint needs storage, replay, or other wire compatibility
- **THEN** AIGW SHALL NOT rename its provider identity or encode transport
  behavior in Account metadata

#### Scenario: Reject implicit credential transport

- **WHEN** an imported manifest contains a token, password, authorization
  header, API key, or equivalent credential field
- **THEN** AIGW SHALL reject the manifest without changing local configuration

### Requirement: Independent product authority

AIGW SHALL own only provider configuration, Account credentials, Route
selection, native Codex projection, and the native Claude Code launcher. It MUST
NOT proxy traffic, manage Proxy lifecycle, control unrelated applications, or
rewrite Codex JSONL, SQLite, history, item records, model selection, or metadata.

#### Scenario: Native Codex CLI and Desktop share a home

- **WHEN** AIGW discovers the native Codex Home
- **THEN** it SHALL project its marked selection into that shared `config.toml`
  without editing application-managed history or GUI state

#### Scenario: Codex uses the selected Proxy

- **WHEN** an Account selects the Proxy Responses endpoint
- **THEN** AIGW SHALL treat it as an ordinary endpoint and Claude Code SHALL
  continue using its independently selected Anthropic endpoint

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures that service's HTTP endpoint as an Account
- **THEN** AIGW SHALL treat it exactly as an external endpoint and SHALL NOT
  acquire lifecycle or state ownership over the service

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, fingerprint,
signing program, trust anchor, foreign-application private path, or external
service lifecycle. CI SHALL provide trust material only from protected context.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, host, or
  Forge
- **THEN** build, verification, setup, repair, and uninstall SHALL not require
  the original contributor's machine, account, key, IDE, or workstation state

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner; composition roots SHALL
only assemble those owners. Source gates MUST reject compatibility shims,
forwarding wrappers, alias-only packages, forbidden product references,
unmanaged flat structure, host-dependent policy paths, and statement coverage
of 95 percent or less for any package or the aggregate.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change introduces a forbidden owner shape or lowers any package or
  aggregate coverage to 95 percent or less
- **THEN** local and hosted verification SHALL fail before publication

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another
  host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows

### Requirement: Complete Forge commit provenance

Each Forge's protected publication context SHALL provide its actor, signer,
trust anchor, and remote coordinates. Every commit reachable from a published
branch tip MUST store that Forge actor as author and committer and verify with
the explicit trust input. Source and CI SHALL use current stable supported build
inputs owned by `go.mod` and CI policy.

#### Scenario: Reachable history contains invalid provenance

- **WHEN** any reachable commit has a different author or committer, lacks a
  trusted signature, or is hidden behind a floor or mailmap
- **THEN** commit-provenance verification SHALL fail and publication SHALL stop

#### Scenario: Stable inputs advance

- **WHEN** a newer stable supported compiler, module, or CI action is selected
- **THEN** the owning source SHALL record the exact version and all native and
  repository gates SHALL pass before publication
