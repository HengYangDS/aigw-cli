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

For a provider-prefixed Codex model whose base slug has exactly one match in the
installed client's bundled model table, AIGW SHALL derive a complete catalog
mirror with namespace aliases, bind it to the exact client version and
executable digest, and preserve the selected wire model ID. The catalog,
configuration reference, and sidecar SHALL share the existing compensated
projection transaction. AIGW SHALL NOT adopt a user-authored catalog or a
foreign file at its managed path.

#### Scenario: Multi-target projection fails

- **WHEN** any selected target cannot be prepared or committed
- **THEN** AIGW SHALL report failure and restore only artifacts whose postimage
  still belongs to the failing transaction

#### Scenario: Inspect a dry run

- **WHEN** an operator runs synchronization in dry-run JSON mode
- **THEN** AIGW SHALL return the target and action plan without a credential or
  client-lifecycle side effect

#### Scenario: A provider prefixes a known Codex model

- **WHEN** exactly one suffix of the selected model ID matches the client's own
  bundled model table
- **THEN** AIGW SHALL project aliases for the complete bundled table under the
  derived namespace
- **AND** the provider SHALL continue receiving the original selected model ID.

#### Scenario: Client identity or catalog ownership is not provable

- **WHEN** the installed client changes, generation fails, the managed bytes
  drift, or a foreign catalog occupies the managed path
- **THEN** AIGW SHALL refuse unsafe reuse, adoption, overwrite, or deletion
- **AND** SHALL preserve user-owned state.

#### Scenario: A real client qualifies the projection

- **WHEN** a contributor runs the tracked catalog verification command
- **THEN** it SHALL record client version and executable digest
- **AND** SHALL prove the adapted model resolves like its base slug while an
  unknown model retains fallback behavior
- **AND** SHALL send no model request and alter no persistent Codex home.

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

Each behavior and policy SHALL have one semantic owner. Composition roots SHALL
assemble declared owners; source gates SHALL enforce positive package topology,
dependency direction, public surfaces, portability, and the canonical coverage
policy. Compatibility facades and duplicate policy owners SHALL not be retained.

#### Scenario: semantic ownership regresses

- **WHEN** a change violates declared topology or dependency direction, or misses the canonical coverage policy
- **THEN** verification SHALL fail with the exact semantic owner and evidence gap.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change violates declared semantic ownership or dependency direction, or misses the canonical package or aggregate coverage policy
- **THEN** local and hosted verification SHALL fail before publication.

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows.

#### Scenario: No admitted branch authority exists

- **WHEN** no stable tool can measure the complete module once on every supported platform
- **THEN** the branch-coverage gate SHALL remain blocked rather than substituting statement coverage.

#### Scenario: A tool needs shared release policy

- **WHEN** repository release tooling and product upgrade behavior require the same source-validation rule
- **THEN** each validates its own authority-bound inputs without importing another runtime owner.

#### Scenario: A legacy concatenated name remains

- **WHEN** a package appears outside the declared direct-owner topology or a repository tool imports an undeclared product owner
- **THEN** the architecture gate fails with the exact path and dependency.

#### Scenario: an ordinary provider is added

- **WHEN** a provider is added below the existing provider owner without changing topology
- **THEN** the existing positive topology admits it without changing repository-shape policy.

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

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW writes one atomic marked projection
- **AND** does not invent a separate Desktop configuration authority.

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
- **AND** aggregate statement and branch coverage remain strictly above 95 percent
- **AND** every package remains present, executed, and visible with exact ratios
- **AND** every package has current bound raw evidence.

#### Scenario: Another team installs AIGW

- **WHEN** source or a signed artifact is used on a supported host
- **THEN** configuration, planning, synchronization, diagnostics, and upgrade use documented portable inputs
- **AND** no author-specific path, key, email, machine service, or foreign product is required.

### Requirement: Local-first independent publication topology

AIGW SHALL declare one repository-owned local verification command, one
repository-owned local installation command, and independent GitLab and GitHub
publication peers. Each peer SHALL own its remote and CI surface. Publication
admission MUST reject an incomplete declaration and MUST NOT make either Forge
depend on the other.

#### Scenario: One Forge is unavailable

- **WHEN** local verification and one declared Forge remain available
- **THEN** local acceptance and the available Forge publication path remain
  independently operable without querying or mutating the unavailable Forge

#### Scenario: The canonical specification is verified

- **WHEN** the repository architecture gate reads the product-control-plane
  specification
- **THEN** it SHALL find exactly one terminal newline
- **AND** the local-first independent publication requirement SHALL remain
  unchanged

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

### Requirement: Terminal candidate integration is exact and local

A proven work lane SHALL advance the local candidate only through explicit
compare-and-swap authority bound to the complete accumulated lane delta.

#### Scenario: The candidate remains the observed ancestor

- **WHEN** full proof passes for the clean archived work-lane HEAD
- **THEN** ETHOS SHALL move `candidate/dev` only from the previously observed ref
- **AND** any candidate, Lease, tree, scope, or proof drift SHALL fail closed
- **AND** no remote Forge SHALL be queried or mutated.

### Requirement: Terminal local release readiness

AIGW SHALL admit a local release candidate only when canonical specifications
contain no placeholder authority, every direct repository dependency is current
and stable, transitive selection remains owned by those direct dependencies,
aggregate statement and branch coverage remain strictly above 95 percent with
current bound evidence, every package remains present and executed, the native source gate passes,
and the complete release matrix is
reproducible and installable. Hosted CI, Forge publication, released-asset
installation, and lane retirement SHALL consume the archived result rather than
become prerequisites of the Change that produces it.

#### Scenario: A stable direct dependency update is available

- **WHEN** the declared Go toolchain reports a newer stable direct module version
- **THEN** `go.mod` and `go.sum` SHALL be refreshed together
- **AND** the complete native source gate SHALL pass before integration

#### Scenario: Only an unneeded transitive update is reported

- **WHEN** the module query reports a newer transitive version but `go mod why`
  shows that the main module does not need that module
- **THEN** AIGW SHALL leave selection with the direct dependency owner
- **AND** SHALL NOT add an explicit pin merely to display the newest version

#### Scenario: A canonical document contains placeholder authority

- **WHEN** a specification purpose remains `TBD` or describes generation history
- **THEN** terminal closeout SHALL fail until the purpose states current product
  semantics directly

#### Scenario: Protected branches are projected

- **WHEN** the operator selects `main` for a Forge identity projection
- **THEN** every `main` and `dev` precondition SHALL pass before publication
- **AND** one atomic push SHALL advance both protected branches or neither
- **AND** only an explicit `proposal/*` selection MAY use single-branch projection
- **AND** candidate, work, or arbitrary branches SHALL be rejected.

#### Scenario: External delivery follows local readiness

- **WHEN** the Change has passed exact-HEAD proof and has been archived and landed
- **THEN** native macOS, Linux, and Windows hosted verification MAY consume that
  exact accepted result
- **AND** GitLab and GitHub MAY publish it independently after their own gates
- **AND** released-asset installation and governed lane retirement occur only
  after the corresponding external evidence exists.

### Requirement: Local release-root promotion

AIGW SHALL advance local release `main` from accepted `dev` only through one
explicit exact compare-and-swap transaction. The transaction SHALL verify a
clean repository, exact observed `main` and `dev` commits, and ancestry before
moving `main`. It SHALL NOT push a remote, create a tag, publish a release, or
change `dev`.

#### Scenario: Accepted dev is ready for release

- **WHEN** `main` and `dev` match the operator's exact observations
- **AND** `main` is an ancestor of `dev`
- **THEN** one compare-and-swap SHALL advance `main` to exactly `dev`
- **AND** `dev` SHALL remain unchanged.

#### Scenario: Release coordinates drift

- **WHEN** observed `main` or `dev` differs from the supplied coordinate
- **THEN** promotion SHALL fail before any ref update.

#### Scenario: Release history diverges

- **WHEN** local `main` is not an ancestor of accepted `dev`
- **THEN** promotion SHALL fail before any ref update.

#### Scenario: Release main already equals accepted dev

- **WHEN** exact `main` and `dev` identify the same commit
- **THEN** promotion SHALL report an idempotent success without changing refs.

### Requirement: Native cross-platform release admission

AIGW SHALL require native source verification on Windows, Linux, and macOS
before a trusted release. The module aggregate SHALL maintain statement and
branch coverage strictly above 95 percent under the single repository coverage
policy. Every production package SHALL appear
in current evidence without platform exclusions or duplicated test stacks.

#### Scenario: A platform exposes an unobserved package or failure boundary

- **WHEN** one native platform reports a wholly unexecuted production package or
  an aggregate ratio at or below the floor
- **THEN** a portable regression SHALL exercise a real behavior or failure
  boundary
- **AND** the coverage policy SHALL remain unchanged
- **AND** all native platforms SHALL execute the same source test suite.

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish exactly one token-free team manifest containing
the reviewed Accounts, Profiles, and recommended client Routes. It SHALL be
directly consumable by `aigw setup --from` and SHALL NOT contain fictitious
providers, credentials, workstation paths, or a parallel example manifest.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the reviewed DMXAPI, AIHubMix, and UCloud profiles
- **AND** SHALL request or reuse Tokens outside the manifest
- **AND** SHALL recommend GPT-5.6 Sol for Codex and Claude Fable 5 for Claude
