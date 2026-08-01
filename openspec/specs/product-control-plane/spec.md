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

### Requirement: Portable source

Product source SHALL NOT encode a personal identity, home directory, private
Forge coordinate, local checkout path, credential, signing key, signing
program, or trust anchor. Publication context SHALL remain external to reusable
source.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, or Forge
- **THEN** ordinary build and source verification SHALL not require the original
  contributor's machine, path, account, key, fingerprint, or remote

### Requirement: Complete Forge commit provenance

Each Forge's protected publication context SHALL provide its actor, signer,
trust anchor, and remote coordinates. Every commit reachable from a published
branch tip MUST store that Forge actor as both author and committer and MUST
verify with the Forge's explicit trust input. A commit floor, mailmap, or
suffix-only exception MUST NOT weaken this invariant.

#### Scenario: Reachable history contains invalid provenance

- **WHEN** any commit reachable from the selected revision has a different
  author or committer, lacks a trusted signature, or is hidden behind a floor or
  mailmap
- **THEN** commit-provenance verification SHALL fail and publication SHALL stop

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

### Requirement: Deterministic local verification

Local verification MUST use controlled fixtures rather than an undeclared
public network dependency.

#### Scenario: A local test depends on external state

- **WHEN** a verification test would contact a public endpoint instead of its
  controlled fixture
- **THEN** local verification SHALL fail or the fixture SHALL intercept the
  exact request without public network I/O

### Requirement: Source-bound quantitative evidence

A dated quantitative observation MUST identify its source revision and tree,
retain its numerator and denominator, and derive rather than independently
assert its displayed percentage.

#### Scenario: Quantitative evidence is incomplete or inconsistent

- **WHEN** dated evidence omits its source identity or raw counts, its percentage
  does not match those counts, or its claim digest does not match the record
- **THEN** governance verification SHALL fail before promotion

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

### Requirement: Semantic Forge history projection

A Forge-specific history projection MUST preserve every source commit's tree,
exact message bytes, author and committer timestamps, ordered parents, and
merge topology while replacing only author identity, committer identity,
signature, and parent object references required by the target graph. The
projection MUST be constructed and verified in an isolated object database
before a canonical or remote ref is changed.

#### Scenario: Replay complete history into another Forge identity

- **WHEN** an authorized publication operation projects a source graph into a
  target Forge identity domain
- **THEN** the target graph SHALL have one mapped commit per source commit, the
  same ordered semantic history, and a trusted target-Forge signature on every
  mapped commit

#### Scenario: Projection cannot prove exact semantics

- **WHEN** a source commit lacks a mapped parent, message bytes or timestamps
  differ, parent order or merge arity changes, object storage is shared, or a
  generated signature does not verify
- **THEN** projection SHALL fail before changing any canonical or remote ref

### Requirement: Recoverable published-history repair

An explicitly authorized repair of published history MUST capture immutable
recovery evidence before mutation. Local ref replacement MUST compare the
expected old object, and remote replacement MUST use the captured remote tip as
a lease.

#### Scenario: A remote advances during prepared repair

- **WHEN** a branch or tag no longer equals the old object captured by the
  recovery record
- **THEN** replacement SHALL stop without overwriting that ref and the repair
  SHALL remain incomplete

### Requirement: Atomic published-history replacement

An authorized repair MUST treat all affected branches, provider-native
annotated tags, releases, hosted CI, release assets, integrity records, and
active commit-bound evidence as one fail-closed operation. Completion MUST NOT
be claimed while an affected Forge still exposes invalid or mixed provenance.

#### Scenario: Both Forge graphs have been replaced

- **WHEN** every affected ref maps to a verified Forge-specific graph
- **THEN** completion SHALL additionally require exact-tip hosted CI, rebuilt
  provider-native releases, matching cross-Forge asset digests, refreshed
  active evidence bindings, and a verified recovery record
