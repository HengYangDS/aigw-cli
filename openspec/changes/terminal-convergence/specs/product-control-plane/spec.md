## MODIFIED Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independently discovered, planned, and projected
through their public configuration contracts. A missing client SHALL remain
untouched. A future client SHALL require a new explicit adapter rather than
reuse a current client's files or semantics.

#### Scenario: One admitted client is absent

- **WHEN** Codex or Claude Code is not installed or configured
- **THEN** AIGW plans and applies only the discovered client
- **AND** creates no placeholder home, shell interception, or foreign state.

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW writes one atomic marked projection
- **AND** does not invent a separate Desktop configuration authority.

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another API-capable agent is later supported
- **THEN** one explicit adapter maps the existing provider-neutral model to its public contract
- **AND** provider and current-client packages remain unchanged.

### Requirement: Declarative ordinary provider extension

An ordinary provider SHALL be introduced through the provider-neutral manifest
and optional diagnostic registry. Adding a provider SHALL not require edits to
Codex, Claude Code, CLI command routing, release, or repository policy.

#### Scenario: A synthetic provider is imported

- **WHEN** a valid provider manifest supplies endpoint, protocol, models, and credential reference
- **THEN** AIGW can plan its Routes and client projections without source changes
- **AND** provider-specific diagnostics remain optional.

#### Scenario: An endpoint needs Responses compatibility

- **WHEN** an Account selects an external compatibility endpoint
- **THEN** AIGW treats it as an ordinary endpoint
- **AND** does not install, configure, start, stop, or verify that service.

### Requirement: Portable source and user contract

Product behavior SHALL contain no personal identity, local checkout path,
ambient credential, Forge dependency, Workstation dependency, or foreign
application assumption. The installed `aigw` command SHALL be the user surface;
repository tools remain developer surfaces.

#### Scenario: An operator selects the environment secret backend

- **WHEN** an operator explicitly chooses process-environment credentials
- **THEN** AIGW resolves only the declared credential reference for that execution
- **AND** does not persist the secret or copy it into generated configuration.

#### Scenario: A contributor verifies rc.80 on a supported host

- **WHEN** a contributor runs the repository-owned verification on macOS, Linux, or Windows
- **THEN** the same declared product and quality contracts execute from repository inputs
- **AND** no developer-global tool, personal path, or foreign product is required.

#### Scenario: Another team installs AIGW

- **WHEN** source or a signed artifact is used on a supported host
- **THEN** configuration, planning, synchronization, diagnostics, and upgrade use documented portable inputs
- **AND** no author-specific path, key, email, or machine service is required.
