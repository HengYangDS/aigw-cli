## Purpose

Define AIGW CLI as a portable provider control plane with explicit authority,
transactional client projections, and no ownership of API traffic or sessions.
## Requirements
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
Forge coordinate, local checkout path, credential, signing key, fingerprint,
signing program, trust anchor, foreign-application private path, or external
service lifecycle. CI SHALL provide trust material only from protected context.

#### Scenario: Build in another team environment

- **WHEN** the repository is cloned under a different user, directory, host, or
  Forge
- **THEN** build, verification, setup, repair, and uninstall SHALL not require
  the original contributor's machine, account, key, IDE, or workstation state

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

### Requirement: Hosted evidence identity is Forge-portable
A hosted governance job SHALL validate tracked evidence by its recorded content
tree within the current Forge's own history.

#### Scenario: Evidence records a commit from the peer Forge
- **WHEN** tracked evidence names a commit object absent from the current Forge
- **THEN** the job SHALL accept the evidence only if the recorded tree exists in
  the current `HEAD` ancestry
- **AND** it SHALL NOT fetch the peer Forge or require a cross-Forge commit map.

#### Scenario: The recorded commit object is locally available
- **WHEN** the recorded commit resolves in the current repository
- **THEN** its tree SHALL equal the recorded tree
- **AND** the recorded tree SHALL still exist in the current `HEAD` ancestry.

### Requirement: Native client fixtures are repository-controlled
Cross-platform tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: Windows tests an unreadable Claude executable
- **WHEN** native Windows verification exercises client-executable read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.

### Requirement: Portable source and user contract

AIGW SHALL build and verify from its own repository and SHALL document every
public setup input without requiring another repository, a workstation-local
path, or an undocumented environment variable.

#### Scenario: An operator selects the environment secret backend

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** AIGW reads `AIGW_TOKEN_<ACCOUNT>` slots without writing them
- **AND** the README explains the behavior without exposing a real token.

#### Scenario: A contributor verifies rc.80 on a supported host

- **WHEN** native Linux, Windows, or macOS verification runs
- **THEN** repository-controlled fixtures exercise equivalent product meaning
- **AND** every package and the aggregate remain strictly above 95% coverage.

### Requirement: Local-first independent publication topology

AIGW SHALL declare one executable local verification command, one executable
local installation command, and separate GitLab and GitHub remotes and CI
surfaces. Publication admission MUST reject an incomplete declaration and MUST
NOT require either Forge to operate the other.

#### Scenario: One Forge is unavailable

- **WHEN** local verification and one declared Forge remain available
- **THEN** local acceptance and that Forge's publication path SHALL remain
  independently inspectable without mutating or querying the unavailable Forge
