## Purpose

Define AIGW CLI as a portable provider control plane with explicit authority,
transactional client projections, and no ownership of API traffic or sessions.

## Requirements

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

### Requirement: Portable single-backend Token storage

AIGW SHALL select exactly one Account Token backend for each installation.
Automatic selection SHALL probe the native credential service on macOS, Linux,
and Windows. When that service is unavailable, AIGW SHALL pin one
platform-protected fallback without searching both stores. The Windows fallback
SHALL protect Token bytes with current-user DPAPI before persistence. Explicit
`keyring` selection SHALL fail closed, and explicit `env` selection SHALL remain
read-only and process-scoped.

#### Scenario: Native credential service is unavailable

- **WHEN** the native credential service cannot be used on a supported platform
- **THEN** automatic selection SHALL pin exactly one platform-protected fallback
- **AND** setup with one Provider Account Token SHALL not require another Account
- **AND** AIGW SHALL NOT search both native and fallback stores.

#### Scenario: Windows fallback persists a Token

- **WHEN** automatic selection falls back on Windows
- **THEN** the stored Token bytes SHALL be protected by current-user DPAPI
- **AND** a subsequent process SHALL recover the Token through the pinned backend
- **AND** explicit `keyring` selection SHALL remain fail-closed.

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
- **THEN** it SHALL use the client's public model-catalog command surface
- **AND** SHALL record the exact client version and executable digest
- **AND** SHALL prove the generated alias is present in the effective catalog
- **AND** SHALL prove all alias metadata except `slug` equals the bundled base
  metadata
- **AND** SHALL prove the bundled catalog did not already contain that alias
- **AND** SHALL prove an unrelated unknown entry remains absent
- **AND** SHALL send no model request and alter no persistent Codex home.

#### Scenario: The client no longer exposes a compatible catalog surface

- **WHEN** the installed client rejects the command or returns an invalid catalog
- **THEN** verification SHALL fail explicitly
- **AND** SHALL NOT substitute prompt shape, item counts, private debug settings,
  or a locally maintained model schema.

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

Every published branch tip MUST identify the exact locally constructed product
commit. The complete reachable product history MUST preserve its original
author and committer identities and MUST verify with the explicit product trust
input. A Forge protected context SHALL provide only its independent transport
credential, remote coordinate, and hosted verification; it SHALL NOT construct,
rewrite, or re-sign a product commit.

#### Scenario: Reachable history contains invalid provenance

- **WHEN** any reachable commit has a different author or committer from its
  local product object, lacks a trusted signature, or is hidden behind a floor
  or mailmap
- **THEN** commit-provenance verification SHALL fail and publication SHALL stop.

#### Scenario: The same product commit reaches two peers

- **WHEN** GitLab and GitHub publish one accepted local revision
- **THEN** both branch tips SHALL equal the local commit OID exactly
- **AND** each peer MAY use different transport credentials without changing
  the product object.

#### Scenario: Stable inputs advance

- **WHEN** a newer stable supported compiler, module, or CI action is selected
- **THEN** its repository-owned authority SHALL record the exact version
- **AND** all native and repository gates SHALL pass before publication.

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

### Requirement: Recoverable published-history repair

An explicitly authorized repair of a divergent published ref MUST capture its
exact current object before mutation. Remote replacement MUST use that object
as a compare-and-swap lease, and the peer MUST be re-observed after the push.

#### Scenario: A remote advances during prepared repair

- **WHEN** a branch or tag no longer equals the object captured by the repair
- **THEN** replacement SHALL stop without overwriting that ref
- **AND** a fresh observation SHALL be required.

### Requirement: Atomic published-history replacement

An authorized cutover MUST project one exact local product object to every
selected ref in a single atomic peer transaction. GitLab and GitHub SHALL be
cut over and verified independently; neither peer SHALL be read as authority
for the other.

#### Scenario: A protected peer is cut over

- **WHEN** exact old tips, the signed local object, and temporary destructive
  authorization are all present
- **THEN** remote `main` and `dev` SHALL move atomically to that exact object
- **AND** force-push authorization SHALL be restored to disabled immediately
  after re-observation.

#### Scenario: Both Forge graphs have been replaced

- **WHEN** both peers complete an explicitly authorized historical cutover
- **THEN** each selected branch and formal release tag SHALL equal the same
  local product objects exactly
- **AND** completion SHALL additionally require exact-tip hosted CI, matching
  asset digests, refreshed active evidence bindings, and the cutover receipts.

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
Credential retrieval for an admitted client SHALL resolve that client's active
Profile and selected Account through the single AIGW Token authority. Claude
Code's owned credential helper and an explicit Codex native-provider
authentication command SHALL invoke the installed AIGW executable by an
absolute, shell-safe path and SHALL NOT place a plaintext Token in client
configuration. AIGW-owned Claude invocations SHALL use Claude Code's
non-experimental compatibility mode so an ordinary admitted
Anthropic-compatible endpoint is not required to implement optional beta
negotiation. Adding a future client MUST NOT change provider policy or another
adapter.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the exact installed AIGW executable
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** an enabled Codex Profile selects an explicit native provider identity
- **THEN** the projected authentication command SHALL invoke the exact installed
  AIGW executable for the Codex client
- **AND** it SHALL return only the Token of the Account selected by the active
  Codex Route
- **AND** the projected Codex configuration SHALL contain no plaintext Token

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** AIGW launches Claude Code for an admitted Claude Profile
- **THEN** the process SHALL disable optional experimental beta negotiation
- **AND** an ambient compatibility value SHALL NOT override the AIGW-owned value
- **AND** the setting SHALL remain process-local

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled adapter,
  an unresolved Route, or an Account without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, Proxy behavior, command roots,
  or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected Profile once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

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

Hosted governance SHALL bind tracked evidence to the exact product commit and
tree. Because every publication peer receives the same object, verification
MUST NOT use tree-only substitution, peer fetches, or commit maps.

#### Scenario: Evidence names the accepted product commit

- **WHEN** a hosted job verifies tracked evidence
- **THEN** the recorded commit and tree SHALL equal the current product object
  and its tree exactly.

#### Scenario: Evidence records a commit from the peer Forge

- **WHEN** tracked evidence names a commit observed on either peer
- **THEN** that commit SHALL resolve to the exact local product object
- **AND** the job SHALL NOT fetch the other peer or consult a commit map.

#### Scenario: The recorded commit object is locally available

- **WHEN** the recorded commit resolves in the current repository
- **THEN** its tree SHALL equal the recorded tree
- **AND** the object SHALL still exist in the current `HEAD` ancestry.

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

AIGW SHALL have one local product-object authority and zero, one, or two
independent optional GitLab and GitHub publication peers. Each peer SHALL own
only its remote transport, hosted CI, Release record, and assets. Publication
MUST push the exact locally signed commit and annotated tag and MUST NOT query,
rewrite, or depend on the other peer.

#### Scenario: No Forge is configured

- **WHEN** the repository is used with zero remote peers
- **THEN** verification, signing, build, installation, upgrade, uninstall, and
  runtime acceptance SHALL remain complete locally.

#### Scenario: One Forge is unavailable

- **WHEN** local verification and one declared peer remain available
- **THEN** the available publication path SHALL remain independently operable
- **AND** the unavailable peer SHALL be reported as incomplete rather than
  weakening or blocking the local product lifecycle.

#### Scenario: Main is published

- **WHEN** an operator selects local `main`
- **THEN** one atomic peer push SHALL set remote `main` and `dev` to that exact
  commit or change neither
- **AND** only an explicit `proposal/*` selection MAY publish one matching ref
- **AND** `candidate/*`, `work/*`, and arbitrary branches SHALL not be
  publication inputs.

#### Scenario: The canonical specification is verified

- **WHEN** the repository architecture gate reads the product-control-plane
  specification
- **THEN** it SHALL find exactly one terminal newline
- **AND** the local-first exact-object publication requirement SHALL remain
  unchanged.

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
contain no placeholder authority, every direct repository dependency is
current and stable, aggregate statement and branch coverage remain strictly
above 95 percent with current bound evidence, every package remains present and
executed, the native source gate passes, and the complete release matrix is
reproducible and installable. Hosted CI, peer publication, released-asset
installation, and lane retirement SHALL consume the archived local result
rather than become prerequisites of the Change that produces it.

#### Scenario: A stable direct dependency update is available

- **WHEN** the declared Go toolchain reports a newer stable direct module
  version
- **THEN** `go.mod` and `go.sum` SHALL be refreshed together
- **AND** the complete native source gate SHALL pass before integration.

#### Scenario: Only an unneeded transitive update is reported

- **WHEN** the module query reports a newer transitive version but `go mod why`
  shows the main module does not need it
- **THEN** AIGW SHALL leave selection with the direct dependency owner
- **AND** SHALL NOT add an explicit pin merely to display the newest version.

#### Scenario: A canonical document contains placeholder authority

- **WHEN** a specification purpose remains `TBD` or describes generation
  history
- **THEN** terminal closeout SHALL fail until the purpose states current
  product semantics directly.

#### Scenario: Protected branches are projected

- **WHEN** a proven accepted local `main` is selected for one peer
- **THEN** its signature and exact object SHALL be verified before publication
- **AND** remote `main` and `dev` SHALL advance atomically to that object
- **AND** the other peer SHALL not be queried or mutated.

#### Scenario: External delivery follows local readiness

- **WHEN** the Change has passed exact-HEAD proof and has been archived and
  landed
- **THEN** native hosted verification and each optional peer MAY independently
  consume that exact accepted result
- **AND** released-asset installation and governed lane retirement occur only
  after their corresponding external evidence exists.

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
directly consumable by `aigw setup --from` without requiring a Token or an
installed client and SHALL NOT contain fictitious providers, credentials,
workstation paths, or a parallel example manifest. When one or more Accounts
are connected, AIGW SHALL preserve each reviewed Route's client and model
intent while selecting an equivalent Profile owned by an available Account. A
later `aigw sync` SHALL re-evaluate the Accounts currently available through
the configured credential backend, select compatible Routes, discover
supported clients, and project only AIGW-owned configuration without exposing,
moving, or rebinding credential values.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the reviewed DMXAPI, AIHubMix, and UCloud profiles
- **AND** SHALL request or reuse Tokens outside the manifest
- **AND** SHALL recommend GPT-5.6 Sol for Codex and Claude Fable 5 for Claude.

#### Scenario: No Account is connected during import

- **WHEN** a user imports the team manifest without supplying a Token
- **THEN** every reviewed Account and Profile SHALL be retained
- **AND** no client installation or credential SHALL be required
- **AND** the next action SHALL enumerate the compatible Account connection
  choices without making one Account mandatory.

#### Scenario: One Provider Account is connected

- **WHEN** a user imports the team manifest with exactly one available Account Token
- **THEN** setup SHALL succeed without Tokens for other Accounts
- **AND** each route SHALL select a compatible Profile owned by the connected Account
- **AND** selection SHALL preserve the reviewed model when that Account offers it
- **AND** a lexical fallback MAY be used only when no equivalent model exists.

#### Scenario: A compatible Account becomes available after import

- **WHEN** setup retained the reviewed catalogue without a connected Account
- **AND** a Token for any compatible Account later becomes available through
  the configured credential backend
- **THEN** `aigw sync` SHALL select compatible Profiles owned by that Account
- **AND** SHALL preserve the reviewed client and model intent
- **AND** SHALL NOT require Tokens for other Accounts.

#### Scenario: A supported client is installed later

- **WHEN** setup completed before Codex or Claude Code was installed
- **AND** an available Account has a compatible route
- **THEN** `aigw sync` SHALL discover and project that client
- **AND** SHALL NOT require, replace, or expose any Token
- **AND** SHALL leave absent clients untouched.

### Requirement: Profile-scoped Codex native provider

A Profile SHALL remain the sole owner of its client, Account, model, and
optional client-native provider selection. A missing Codex provider selection
SHALL resolve to the canonical `aigw` provider. Provider selection SHALL NOT be
inferred from an Account, endpoint, proxy implementation, or client-private
state.

#### Scenario: Explicit Codex provider

- **WHEN** a Codex-scoped Profile declares a safe `model_provider`
- **THEN** its resolved Runtime carries that exact provider identity
- **AND** Codex receives one attributed native provider table using the
  Profile's Account endpoint

#### Scenario: Default Codex provider

- **WHEN** a Codex-scoped Profile omits `model_provider`
- **THEN** its Runtime resolves the canonical `aigw` provider
- **AND** the existing AIGW projection remains byte-compatible

#### Scenario: Provider ownership is narrow

- **WHEN** a non-Codex Profile or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection

### Requirement: Provider-owned Codex authentication

The canonical `aigw` provider SHALL use generic Codex login and the AIGW model
catalogue. An explicit native provider SHALL instead use Codex command-backed
authentication with the absolute AIGW executable and SHALL NOT project a Token,
an environment-key alternative, `requires_openai_auth`, or the AIGW catalogue.

#### Scenario: Native provider projection

- **WHEN** AIGW synchronizes an explicit native provider
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes `aigw credential codex`
- **AND** generic Codex login is not requested

#### Scenario: Return to the default provider

- **WHEN** a Profile changes from an explicit provider to the default provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** generic Codex authentication is rebound

### Requirement: Client readiness is evidence-bounded

AIGW SHALL report projection readiness and native client authentication as
separate facts and SHALL NOT describe Codex as fully ready unless the public
Codex status command proves authentication for the selected target.

#### Scenario: Projection exists without native authentication

- **WHEN** the Codex projection, Token, and endpoint are available
- **AND** the public Codex login-status command does not prove authentication
- **THEN** status reports the projection as ready
- **AND** reports native authentication as required
- **AND** names aigw adapter auth codex as the next action

#### Scenario: Native authentication is proved

- **WHEN** the public Codex login-status command succeeds for every selected target
- **THEN** status may report Codex as locally ready
- **AND** the JSON projection records native authentication as present

### Requirement: Native released-artifact lifecycle acceptance

AIGW SHALL prove the public installed-program lifecycle on macOS, Linux, and
Windows using portable archives and checksum manifests with the same shape as
published release assets. Each native journey SHALL install an older program,
update to a newer program, roll back to the immediate predecessor, recover
forward to the newer program, and uninstall. The journey SHALL preserve user
configuration and credentials according to the existing retention contract and
SHALL leave no installed executable, rollback copy, staging file, or other
owned lifecycle residue after uninstall.

#### Scenario: Released program completes the reversible lifecycle

- **WHEN** native acceptance runs on a supported operating system
- **THEN** the installed older program updates from a verified newer portable archive
- **AND** the installed program reports the newer version
- **AND** rollback restores the older version
- **AND** a second verified update restores the newer version
- **AND** uninstall removes the executable, its single rollback copy, and all staging residue
- **AND** retained configuration and credentials remain available as declared.

#### Scenario: Lifecycle evidence uses the product command plane

- **WHEN** native acceptance exercises update, rollback, forward recovery, or uninstall
- **THEN** it SHALL invoke the public AIGW command for that transition
- **AND** SHALL NOT substitute a package-level helper, platform-specific script state machine, Forge API, or external Proxy lifecycle.

### Requirement: Composable extension boundary

AIGW SHALL keep local configuration and client projection independent from API
traffic processing. A compatible endpoint or model SHALL enter as Account data;
a distinct credential exchange SHALL extend Account authentication; a new local
client SHALL enter through one complete Client Adapter; and incompatible wire
behavior SHALL remain in an independently operated data-plane product. An
external gateway or compatibility service MUST remain an optional Account
endpoint and MUST NOT become an AIGW runtime dependency.

A mature dependency SHALL be admitted only when it preserves these authority
boundaries and removes more owned implementation, tests, dependencies, and
operational states than it introduces.

#### Scenario: Add a compatible Provider endpoint

- **WHEN** an endpoint satisfies an admitted client protocol and existing
  Account authentication contract
- **THEN** an operator SHALL add it through configuration data
- **AND** AIGW SHALL NOT add provider-name branching or a provider-specific
  runtime package.

#### Scenario: Add a client integration

- **WHEN** a new local client requires AIGW-managed configuration
- **THEN** it SHALL be admitted through its own discovery, planning, guarded
  projection, verification, rollback, and uninstall boundary
- **AND** it SHALL NOT reuse another client's conditional path or private state.

#### Scenario: Compose with a traffic gateway

- **WHEN** an operator selects a general gateway or narrow compatibility
  service
- **THEN** AIGW SHALL model its URL as an ordinary Account endpoint
- **AND** SHALL NOT install, supervise, configure, embed, or copy the service's
  traffic policy.

#### Scenario: Evaluate a mature dependency

- **WHEN** a library or framework is proposed for an AIGW-owned boundary
- **THEN** admission SHALL demonstrate a net reduction in owned complexity
- **AND** popularity or feature count alone SHALL NOT justify adoption.
