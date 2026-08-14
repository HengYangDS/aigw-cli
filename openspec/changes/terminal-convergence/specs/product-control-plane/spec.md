## MODIFIED Requirements

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

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW writes one atomic marked projection
- **AND** does not invent a separate Desktop configuration authority.

### Requirement: Declarative ordinary provider extension

An ordinary provider SHALL be admitted through the provider-neutral manifest,
token-free Account, endpoint, Profile, and Route data, and an optional diagnostic
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

### Requirement: Portable source and user contract

Product behavior SHALL contain no personal identity, local checkout path,
ambient credential, Forge dependency, Workstation dependency, foreign repository,
foreign application assumption, or undocumented environment variable. AIGW SHALL
build and verify from its own repository. The installed `aigw` command SHALL be
the user surface; repository tools remain developer surfaces.

#### Scenario: An operator selects the environment secret backend

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** AIGW reads `AIGW_TOKEN_<ACCOUNT>` slots without writing them
- **AND** the README explains the behavior without exposing a real token.

#### Scenario: A contributor verifies rc.80 on a supported host

- **WHEN** native Linux, Windows, or macOS verification runs
- **THEN** repository-controlled fixtures exercise equivalent product meaning
- **AND** statement and branch coverage for every package and the aggregate
  remain strictly above 95 percent.

#### Scenario: Another team installs AIGW

- **WHEN** source or a signed artifact is used on a supported host
- **THEN** configuration, planning, synchronization, diagnostics, and upgrade use documented portable inputs
- **AND** no author-specific path, key, email, machine service, or foreign product is required.

### Requirement: Latest stable repository-owned supply chain

AIGW SHALL lock current stable Go, tool, Action, and release dependencies
through one repository-owned authority for each ecosystem. The declared Go
toolchain and resolver SHALL own transitive closure; local verification and both
Forge projections SHALL consume those declarations rather than duplicate version
literals or compatibility fallbacks.

#### Scenario: A stable transitive update is available

- **WHEN** the Go resolver reports a newer stable transitive dependency
- **THEN** the repository SHALL update `go.mod` and `go.sum` together
- **AND** the complete native gate SHALL pass before integration

#### Scenario: A preceding archive projection changes text layout

- **WHEN** an OpenSpec archive projection leaves a surplus terminal blank line
- **THEN** the same native gate SHALL reject it
- **AND** the active closeout SHALL restore canonical text without weakening policy

#### Scenario: A declared stable dependency advances

- **WHEN** the locked supply chain is refreshed
- **THEN** local development, GitLab, and GitHub resolve the same declared versions
- **AND** obsolete pins and compatibility fallbacks are removed.
