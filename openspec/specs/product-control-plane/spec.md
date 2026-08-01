## Purpose

Define AIGW CLI as a portable provider control plane with explicit authority,
transactional client projections, and no ownership of API traffic or sessions.
## Requirements
### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, Profiles, Routes, endpoints, client model choices,
and explicit Responses storage requirements without requiring a named gateway
product or deployment topology. Configuration manifests MUST remain free of
credentials and MUST require explicit consent before replacing conflicting
local Account or Profile metadata.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports a manifest containing a new HTTPS endpoint and
  selects its Profile
- **THEN** AIGW SHALL route the admitted client without a provider-specific CLI,
  installer, service manager, or source-level product dependency

#### Scenario: Reject implicit credential transport

- **WHEN** an imported manifest contains a token, password, authorization
  header, API key, or equivalent credential field
- **THEN** AIGW SHALL reject the manifest without changing local configuration

### Requirement: Independent product authority

AIGW SHALL own only provider configuration, system-held Account credentials,
Route selection, and marked client projections. It MUST NOT proxy Responses
traffic; install, supervise, or diagnose an external compatibility service;
control an IDE; or rewrite an existing Codex conversation's JSONL, SQLite,
history, item records, model selection, or metadata.

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures that service's HTTP endpoint as an Account
- **THEN** AIGW SHALL treat it exactly as an external endpoint and SHALL NOT
  acquire lifecycle or state ownership over the service

### Requirement: Transactional and inspectable projection

AIGW SHALL prepare and validate every selected client target before mutation,
apply only owned marked projections, and compensate a failed multi-target
operation without overwriting a newer writer. A dry run MUST expose the planned
actions without reading credentials, authenticating, starting a client, or
changing files.

#### Scenario: Multi-target projection fails

- **WHEN** any selected target cannot be prepared or committed
- **THEN** AIGW SHALL report failure and restore only artifacts whose postimage
  still belongs to the failing transaction

#### Scenario: Inspect a dry run

- **WHEN** an operator runs synchronization in dry-run JSON mode
- **THEN** AIGW SHALL return the target and action plan without a credential or
  client-lifecycle side effect

### Requirement: Portable source and release trust

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, or signing key. A Forge's
protected publication context SHALL provide the author, signer, trust anchor,
and remote coordinates needed for its commit and tag provenance.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, or Forge
- **THEN** its source verification and ordinary build SHALL not require the
  original contributor's machine, path, account, or key

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner; composition roots SHALL
only assemble those owners. Source gates MUST reject compatibility shims,
forwarding wrappers, alias-only packages, forbidden product references,
unmanaged flat structure, and statement coverage of 95 percent or less for any
package or the aggregate.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change introduces a forbidden owner shape or lowers any package or
  aggregate coverage to 95 percent or less
- **THEN** local and hosted verification SHALL fail before publication

### Requirement: Documented package ownership

Every non-command production package MUST document its package contract at the
implementation owner.

#### Scenario: Package ownership is undocumented

- **WHEN** a production package omits a `Package <name>` contract or documents
  another package name
- **THEN** architecture verification SHALL fail before publication

### Requirement: Quiet handled failures

Handled CLI failures MUST NOT emit a framework usage banner, warning,
traceback, or false completion message.

#### Scenario: A handled CLI failure occurs

- **WHEN** a command returns an expected operational error
- **THEN** the command SHALL return that error without usage, warning,
  traceback, or completion residue
