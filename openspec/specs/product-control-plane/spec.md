## Purpose

Define AIGW CLI as a portable provider control plane with explicit authority,
transactional client projections, and no ownership of API traffic or sessions.

## Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model Accounts, canonical Models, Routes, Client Bindings,
endpoints, and authentication without provider-specific hacks, named gateways,
topology, or global fallback. Only the current credential-free schema SHALL
execute. Offline validation SHALL serve persistence and import, report the first
failure deterministically, and derive readiness from authentication ownership.

#### Scenario: Diagnose several configuration problems

- **WHEN** the same configuration contains several invalid entries
- **THEN** repeated validation SHALL identify the same first problem
- **AND** the supplied configuration, including unset collections, SHALL remain
  unchanged.

### Requirement: Route and binding compatibility

Every Route SHALL reference one existing Account and one canonical Model and
SHALL declare its admitted protocol interfaces. Every Client Binding SHALL
select one compatible Route and one protocol admitted by both that Route and
the client.

#### Scenario: Import a Route with an incompatible Account

- **WHEN** a Route declares a protocol interface but its Account lacks that
  protocol endpoint
- **THEN** configuration validation and manifest import reject it before writes
- **AND** the error identifies the Route, Account, and missing protocol.

#### Scenario: Save an incompatible Route

- **WHEN** configuration persistence receives a Route whose Account lacks its
  declared protocol endpoint
- **THEN** it rejects the change and preserves the existing configuration file.

#### Scenario: Select independent Claude and Codex services

- **WHEN** an operator selects a Claude Route and a Codex Route
- **THEN** each client SHALL retain its own explicit Route
- **AND** no global default or implicit inheritance SHALL participate in
  resolution, readiness, or projection.

#### Scenario: Interactive selection omits the client

- **WHEN** an interactive operator runs `aigw use <route>` without `--for`
- **THEN** AIGW SHALL offer only admitted clients compatible with that Route
- **AND** non-interactive invocation SHALL require `--for <client>` rather than
  infer a client from the Route.

#### Scenario: Check every enabled client Route

- **WHEN** Claude and Codex are enabled with distinct selected Routes
- **THEN** `aigw check` SHALL validate both effective Routes
- **AND** SHALL derive each AIGW-owned authentication request from that Client Binding's
  selected protocol and authentication mode
- **AND** SHALL not issue an AIGW-owned authentication request for a
  client-native Route
- **AND** SHALL not inspect an unselected historical Route as a fallback
- **AND** MAY coalesce only authentication probes with an identical Account,
  endpoint, and protocol identity.

#### Scenario: Check a client-native Route

- **WHEN** an enabled Route selects a client-native Route whose local client
  projection is valid
- **THEN** `aigw check` SHALL report local readiness without claiming that the
  remote authentication or model request succeeded
- **AND** SHALL provide `aigw verify --for <client>` as the explicit live-proof
  continuation.

#### Scenario: No client is enabled

- **WHEN** configuration is valid but no admitted client Adapter is enabled
- **THEN** `aigw check` SHALL report configuration readiness
- **AND** SHALL not claim that an arbitrary gateway or model is healthy.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `status --json` reports a resolved Route with an endpoint address
- **THEN** it SHALL report `endpoint_configured` as true
- **AND** SHALL not represent that configuration fact as endpoint or transport
  health.

#### Scenario: Report the exact scope of a successful check

- **WHEN** `check --json` finishes its applicable Route checks
- **THEN** `check_passed` SHALL describe command-check success, not inference
  or real-client readiness
- **AND** a valid client-native projection SHALL remain `configured` without
  an AIGW-owned endpoint diagnostic
- **AND** a configured Account-Token Route with a successful endpoint
  diagnostic SHALL be `endpoint_checked`, not generically `ready`
- **AND** human output SHALL describe the same observed scope
- **AND** `next_action` SHALL carry both repair and verification continuations
  without a duplicate `fix` field.

#### Scenario: Read previous local configuration

- **WHEN** AIGW reads its local configuration
- **THEN** it SHALL accept the current schema with explicit per-client Routes
- **AND** an earlier schema SHALL return its actual version and the current
  required version without inferring or migrating Route selection.

#### Scenario: Import a multi-provider team catalogue

- **WHEN** a team manifest declares recommended Routes for admitted clients
- **THEN** setup SHALL retain those recommendations separately from actual
  per-client selections and fill only currently usable unselected Routes
- **AND** neither a global default nor an unavailable recommendation SHALL
  replace an explicit client selection.
- **AND** a future client SHALL remain unselected until its own Route is
  explicitly admitted.

#### Scenario: Diagnose a partially connected team catalogue

- **WHEN** a reviewed catalogue contains multiple Accounts
- **AND** every Account selected by an enabled client Route has its Token
- **THEN** `aigw doctor` SHALL report the credential state as healthy
- **AND** SHALL NOT fail for an unselected Account whose Token is absent.

#### Scenario: Diagnose a selected Account without a Token

- **WHEN** an enabled client Route selects an Account-Token Route whose Token
  is absent
- **THEN** `aigw doctor` SHALL report that Account as unhealthy
- **AND** SHALL provide the account-scoped rotation action.

#### Scenario: Diagnostic outcome is independent of presentation

- **WHEN** `doctor` observes a failed diagnostic or an invalid, degraded,
  unavailable or unclassified admitted-client state
- **THEN** human and JSON output SHALL both return a nonzero exit status
- **AND** JSON SHALL contain `ok: false` and the same `next_action` shown to
  the human reader
- **AND** JSON SHALL remain one document without appended prose or another
  error rendering
- **AND** successfully writing that report SHALL NOT change the diagnosis.

#### Scenario: Deferred clients do not invalidate a sound configuration

- **WHEN** all diagnostics pass and the observed clients are configured,
  endpoint-checked or intentionally deferred
- **THEN** `doctor` SHALL return success in human and JSON modes
- **AND** SHALL not invent a repair continuation.

#### Scenario: Diagnose a client-native Route

- **WHEN** an enabled client Route selects a client-native Route
- **THEN** `status`, `check`, `doctor`, `test`, `credential`, and Route
  inspection SHALL NOT query the AIGW Account Token store for that Route
- **AND** read-only output SHALL identify client-owned authentication and use
  `aigw verify --for <client>` for live proof where applicable.

#### Scenario: Add an ordinary provider

- **WHEN** an operator imports token-free Account and Route data for a new
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

#### Scenario: Codex uses the selected endpoint

- **WHEN** an Account selects an implementation-neutral Responses endpoint
- **THEN** AIGW SHALL treat it as an ordinary endpoint
- **AND** Claude Code SHALL continue using its independently selected Anthropic
  endpoint.

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures an external service HTTP endpoint as an
  Account
- **THEN** AIGW SHALL treat it exactly as an external endpoint
- **AND** SHALL NOT acquire lifecycle or state ownership over that service.

### Requirement: Transactional and inspectable projection

AIGW SHALL atomically prepare and validate selected client targets, mutate only
owned projections, compensate failures only while postimages remain
transaction-owned, and expose credential-free, side-effect-free dry runs. For a
uniquely matched provider-prefixed Codex model, it SHALL derive a full
bundled-catalogue alias mirror bound to client version and digest, preserve the
wire model ID, and update all three atomically. Foreign or user-authored
catalogues MUST remain untouched.

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

#### Scenario: An operator uses the installed command

- **WHEN** another team installs AIGW on a supported host
- **THEN** the installed executable SHALL provide the user command surface
- **AND** repository tools SHALL remain developer-only entrypoints
- **AND** documented configuration, credentials and discovery inputs SHALL not
  require an author-specific path, identity, key, service or foreign product.

#### Scenario: An operator selects environment credentials

- **WHEN** `AIGW_SECRET_BACKEND=env` is selected
- **THEN** the documented Account variable mapping SHALL supply credentials
  without persistence
- **AND** the user guide SHALL explain process inheritance and direct installed
  command invocation without exposing real credentials.

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

- **WHEN** no maintained admitted analyzer can measure a proposed branch metric
- **THEN** the quantitative policy SHALL omit that unsupported claim
- **AND** native statement evidence and complete package observation SHALL remain
  enforced without being relabeled as branch coverage.

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

A quantitative acceptance observation MUST retain its raw counts and bind the
measured source revision, tree, toolchain and policy through the owning
verification record. Displayed percentages SHALL derive from those counts.
Native output and its invocation context MAY jointly carry this evidence;
tracked source SHALL NOT require a self-referential commit identifier or a
parallel claim-digest registry.

#### Scenario: Quantitative evidence is incomplete or inconsistent

- **WHEN** an acceptance observation omits its measured source context or raw
  counts, or its percentage disagrees with those counts
- **THEN** it SHALL NOT establish the claimed quantitative acceptance
- **AND** the native measurement owner SHALL retain the raw failure rather than
  manufacture missing facts in a second record.

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
token-free Account, endpoint, Route, and Route data, and an optional diagnostic
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

Codex and Claude Code SHALL be independent Adapters owning discovery,
projection, authentication, rollback, verification, status, and withdrawal of
AIGW state. Account-Token helpers MUST use the absolute installed AIGW
command and Route without storing Tokens in client files. Client-native
authentication and preferences SHALL remain client-owned. Verification SHALL
use synchronized settings without wrappers. New clients MUST add an Adapter
without changing provider policy or existing Adapters.

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

- **WHEN** an enabled Codex Route selects an explicit native provider identity
- **THEN** its declared authentication mode SHALL determine credential ownership
- **AND** Account-Token authentication SHALL invoke the exact installed AIGW
  helper for only the active Codex Route
- **AND** client-native authentication SHALL project no AIGW Token helper
- **AND** neither mode SHALL write a plaintext Token to public configuration

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** explicit verification invokes Claude Code for an admitted Route
- **THEN** the native client SHALL consume its synchronized settings in bare,
  nonpersistent mode
- **AND** stale AIGW-owned Anthropic environment overrides SHALL be removed
- **AND** unrelated client preferences, including beta controls, SHALL remain
  unchanged rather than becoming a new AIGW authority

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
  fixtures and SHALL NOT change provider policy, external-gateway behavior,
  command roots, or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected Route once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

### Requirement: Independent product composition

AIGW SHALL treat every valid Account endpoint as an endpoint choice, whether it
is a direct provider HTTPS endpoint or an independently operated gateway. AIGW
MUST NOT identify, import, invoke, install, configure, diagnose, reload,
uninstall, or roll back the product behind that endpoint, and the external
gateway MUST NOT acquire AIGW state.

#### Scenario: An Account selects direct HTTPS

- **WHEN** an operator selects a valid direct HTTPS Account endpoint
- **THEN** AIGW SHALL project that endpoint without requiring a local gateway
- **AND** verification SHALL exercise the selected client through that direct
  endpoint

#### Scenario: An Account selects loopback HTTP

- **WHEN** an operator selects a valid loopback endpoint
- **THEN** AIGW SHALL treat it only as an external endpoint without product,
  fixed-port, path, or lifecycle assumptions

#### Scenario: Governed Codex deployment uses an external gateway

- **WHEN** a governed deployment selects gateway endpoints for one or more
  Codex Routes
- **THEN** AIGW SHALL project those endpoints by the same Account and Route
  semantics used for direct HTTPS endpoints
- **AND** runtime evidence SHALL verify each selected client path without giving
  AIGW gateway lifecycle ownership or encoding a fixed product, path, or port

#### Scenario: An external gateway is unavailable

- **WHEN** an independently operated gateway is absent or unhealthy
- **THEN** AIGW SHALL report only the selected endpoint's observed failure
- **AND** SHALL NOT install, start, stop, repair, or reconfigure the gateway

### Requirement: Foreign applications remain independent

AIGW SHALL NOT depend on, discover, configure, align, verify, repair, or control
foreign applications or their private runtime state.

#### Scenario: A foreign application is installed

- **WHEN** AIGW runs on a machine with unrelated applications
- **THEN** product behavior and acceptance SHALL remain independent of those
  applications, their configuration, sessions, caches, and runtime

### Requirement: Hosted evidence identity is Forge-portable

Hosted acceptance SHALL bind its executed input to the exact selected product
commit and tree. Historical observations SHALL retain their original identity
and SHALL NOT become current acceptance merely because their commit is an
ancestor. Independent peers receive the same local objects; tree-only
substitution, peer-specific identity rewriting and commit maps establish no
equivalence.

#### Scenario: Evidence names the accepted product commit

- **WHEN** a hosted job claims current product acceptance
- **THEN** its measured commit and tree SHALL equal the object selected for that
  job, with raw results retained at the job's evidence owner.

#### Scenario: Evidence records a commit from the peer Forge

- **WHEN** evidence names a product commit observed on either peer
- **THEN** that identifier SHALL refer to the same locally constructed object
- **AND** the job SHALL NOT use another peer's rebuilt object or a commit map
  as a substitute for its selected source.

#### Scenario: The recorded commit object is locally available

- **WHEN** a historical observation is retained for comparison
- **THEN** its recorded commit and tree SHALL remain resolvable as observed
- **AND** ancestry or source availability alone SHALL NOT certify a later HEAD.

### Requirement: Native client fixtures are repository-controlled

Cross-platform tests SHALL construct client executables from test-owned
fixtures rather than borrowing unrelated host toolchain executables.

#### Scenario: Windows tests an unreadable Claude executable

- **WHEN** native Windows verification exercises client-executable read failures
- **THEN** the fixture SHALL be an isolated executable controlled by the test
- **AND** the result SHALL not depend on the installed Go toolchain path or contents.

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
- **THEN** the owning direct dependency and native resolver SHALL determine its
  admissible closure; an unused transitive update SHALL NOT create a new pin
- **AND** an admitted graph change SHALL refresh declarations and locks together
  and pass complete native verification before integration

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

- **WHEN** full proof passes for the exact clean work-lane HEAD with valid
  official Change artifacts
- **THEN** local integration SHALL move the declared candidate ref only from
  its previously observed object under current transition authority
- **AND** any candidate, Lease, tree, scope, or proof drift SHALL fail closed
- **AND** no remote Forge SHALL be queried or mutated
- **AND** pending external delivery tasks remain in the same active Change.

### Requirement: Terminal local release readiness

A local release candidate SHALL require complete canonical intent, admitted
stable direct dependencies, faithful quantitative evidence, passing native
source gates, and a reproducible installable matrix. Hosted CI, publication,
installed-asset proof, and lane retirement SHALL consume that accepted result
rather than block its production. Active Change tasks SHALL remain authoritative
until all delivery obligations finish; archive SHALL NOT erase unfinished work.

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

- **WHEN** source has passed exact-HEAD proof and authorized integration
- **THEN** native hosted verification and each optional peer MAY independently
  consume that exact accepted result
- **AND** released-asset installation and governed lane retirement occur only
  after their corresponding external evidence exists
- **AND** the same official task carrier retains pending outcomes; completed
  Change obligations are archived afterward rather than predeclared.

#### Scenario: A clean runner materializes npm tools

- **WHEN** source verification starts without an existing npm installation
- **THEN** bootstrap SHALL install the exact committed npm dependency graph with
  install scripts disabled
- **AND** direct and transitive selections SHALL remain bound to the lockfile
- **AND** registry signatures SHALL verify through the ecosystem verifier.

#### Scenario: The verified release is published

- **WHEN** source and artifact acceptance admit a release for publication
- **THEN** each selected Forge SHALL receive the same locally signed commit,
  annotated tag and immutable asset matrix without re-signing or reconstruction
- **AND** each peer SHALL verify its own publication independently.

### Requirement: Reviewed team configuration is directly consumable

The repository SHALL publish one token-free reviewed manifest directly
consumable by `aigw setup --from` without credentials or installed clients.
Setup and sync SHALL preserve client and model intent, select Routes only
through usable authentication boundaries, project only AIGW-owned state, and
never expose or rebind Tokens. Fictitious providers, workstation paths, and
parallel example manifests SHALL NOT remain.

#### Scenario: Team member imports reviewed settings

- **WHEN** a team member downloads the tracked manifest and runs `aigw setup --from`
- **THEN** AIGW SHALL import the Accounts and Routes from the reviewed
  `manifests/team.toml` without a second provider or model-name policy
- **AND** required Account Tokens SHALL remain outside the manifest
- **AND** recommended selections SHALL come from that manifest rather than
  duplicated model-version literals in this specification.

#### Scenario: No Account is connected during import

- **WHEN** a user imports the team manifest without supplying a Token
- **THEN** every reviewed Account and Route SHALL be retained
- **AND** no client installation or credential SHALL be required
- **AND** the next action SHALL enumerate the compatible Account connection
  choices without making one Account mandatory.

#### Scenario: One Provider Account is connected

- **WHEN** a user imports the team manifest with exactly one available Account Token
- **THEN** setup SHALL succeed without Tokens for other Accounts
- **AND** each route SHALL select a compatible Route owned by the connected Account
- **AND** selection SHALL preserve the reviewed model when that Account offers it
- **AND** a lexical fallback MAY be used only when no equivalent model exists.

#### Scenario: A compatible Account becomes available after import

- **WHEN** setup retained the reviewed catalogue without a connected Account
- **AND** a Token for any compatible Account later becomes available through
  the configured credential backend
- **THEN** `aigw sync` SHALL select compatible Routes owned by that Account
- **AND** SHALL preserve the reviewed client and model intent
- **AND** SHALL NOT require Tokens for other Accounts.

#### Scenario: A supported client is installed later

- **WHEN** setup completed before Codex or Claude Code was installed
- **AND** an available Account has a compatible route
- **THEN** `aigw sync` SHALL discover and project that client
- **AND** SHALL NOT require, replace, or expose any Token
- **AND** SHALL leave absent clients untouched.

### Requirement: Route-scoped Codex native provider

A Route SHALL remain the sole owner of its client, Account, model, and
optional client-native provider selection. A missing Codex provider selection
SHALL resolve to the canonical `aigw` provider. Provider selection SHALL NOT be
inferred from an Account, endpoint, proxy implementation, or client-private
state. Authentication ownership SHALL be independent of the provider name.

#### Scenario: Explicit Codex provider

- **WHEN** a Codex-scoped Route declares a safe `model_provider`
- **THEN** its resolved Runtime carries that exact provider identity
- **AND** Codex receives one attributed provider table using the Route's
  Account endpoint and declared authentication mode.

#### Scenario: Default Codex provider

- **WHEN** a Codex-scoped Route omits `model_provider`
- **THEN** its Runtime resolves the canonical `aigw` provider
- **AND** Account-Token authentication uses the same command-helper contract
  as an explicit provider.

#### Scenario: Provider ownership is narrow

- **WHEN** a non-Codex Route or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection.

### Requirement: Native released-artifact lifecycle acceptance

On macOS, Linux, and Windows, AIGW SHALL prove the public installed-program
lifecycle using portable archives and checksums shaped like release assets:
install the older program, update, roll back, recover forward, and uninstall.
The journey MUST preserve declared configuration and credentials and finish
without an installed executable, rollback copy, staging file, or other owned
lifecycle residue.

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
- **AND** SHALL NOT substitute a package-level helper, platform-specific script state machine, Forge API, or external-gateway lifecycle.

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic.
Endpoints and models SHALL enter as Account data; existing
client-native authentication SHALL be reused through explicit Routes. New
clients SHALL require complete Adapters, unsupported credential contracts SHALL
require admission, and incompatible wire behavior SHALL remain in a separate
data plane. Gateways SHALL remain optional endpoints. Dependencies MAY be
admitted only when total owned complexity falls.

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

### Requirement: Real Codex client route verification

AIGW SHALL verify a selected Codex Route by running the configured and
admitted Codex executable against one synchronized AIGW projection. A direct
HTTP request made by AIGW itself MUST NOT be accepted as proof of the Codex
client path.

#### Scenario: A synchronized Codex target is verified

- **WHEN** an operator verifies a Codex Route with an available executable,
  synchronized target, and usable Account Token
- **THEN** AIGW SHALL run that executable through its non-persistent execution
  surface using the selected target and Route model
- **AND** SHALL accept the verification only when the client's final response is
  exactly the bounded verification marker

#### Scenario: Several synchronized Codex targets exist

- **WHEN** an operator verifies a Codex Route whose adapter owns several
  synchronized targets
- **THEN** AIGW SHALL deterministically select one target for the live request
- **AND** SHALL NOT duplicate the quota-consuming request for equivalent targets

#### Scenario: Codex verification identity is reported

- **WHEN** a Codex verification succeeds
- **THEN** AIGW SHALL report the executable version and SHA-256 digest measured
  for that invocation
- **AND** SHALL NOT expose the Account Token or model response content

#### Scenario: Codex cannot execute the selected route

- **WHEN** the executable is missing or incompatible, its projection is stale,
  authentication is unavailable, the request exceeds its bounded execution
  time, or the response marker is absent
- **THEN** verification SHALL fail with an actionable error scoped to that
  boundary
- **AND** SHALL NOT write client configuration, operator sessions, conversation
  history, model metadata, or external gateway state

### Requirement: Windows path derivation preserves the selected namespace

Derived configuration, data, credential, client-settings and installation paths
SHALL retain the namespace of the explicitly supplied Windows base. Appending
AIGW-owned path components MUST NOT collapse a UNC or extended-length prefix
into a drive-rooted path. Derivation SHALL remain independent of the observer's
operating system and SHALL NOT access a network share to construct its name.

#### Scenario: A user directory is on a network share

- **WHEN** Windows environment paths identify a UNC share or an extended-length
  UNC or drive namespace
- **THEN** every derived AIGW path retains that namespace and base location
- **AND** native Windows path construction agrees with the derived result for
  the supported base forms.

### Requirement: Model discovery reports catalogue evidence only

`aigw catalog` and `aigw models` SHALL share one Account catalogue observation
path. Route comparison SHALL distinguish listed, not listed, and an unobserved
catalogue with its reason. These commands MUST NOT describe catalogue membership
as inference reachability or native-client readiness. An incomplete or oversized
HTTP response SHALL produce a failed catalogue observation, not partial success.

#### Scenario: A configured model appears in the catalogue

- **WHEN** an authenticated catalogue response lists the configured model ID
- **THEN** `models` reports that ID as listed
- **AND** performs no inference or client invocation.

#### Scenario: A model is absent from a successfully observed catalogue

- **WHEN** a complete catalogue does not list the configured model ID
- **THEN** `models` reports not listed rather than unavailable.

#### Scenario: Catalogue observation fails

- **WHEN** credentials cannot be observed, the endpoint is absent, the request
  fails, or the response is incomplete or oversized
- **THEN** the affected Account has no established model membership
- **AND** its observation reason remains visible without blocking other Accounts.

### Requirement: Control-plane convergence is client-scoped and monotonic

AIGW SHALL derive operational state only from Accounts, client-scoped Routes,
explicit per-client Routes, admitted client Adapters, and the selected
credential backend. Setup, selection, synchronization, and readiness MUST NOT
depend on a global Route, an aggregate selection flag, another client's
Route, or the presence of an external compatibility product.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one Codex Route and one Claude Route in
  separate operations
- **THEN** both per-client Routes remain selected
- **AND** readiness requires no additional aggregate selection operation.

#### Scenario: An ordinary configuration edit affects one client

- **WHEN** an Account or Route edit changes one client's persistent projection
- **THEN** the configuration transaction SHALL plan and apply only the affected
  client set in admission order
- **AND** another client's external edits and ownership state SHALL remain
  byte-identical, without being inspected as a prerequisite for that edit.

#### Scenario: A shared edit affects multiple clients

- **WHEN** an edit changes several client projections
- **THEN** all affected clients SHALL participate in the same guarded
  transaction and existing reverse compensation contract
- **AND** explicit synchronization or repair SHALL still reconcile its entire
  requested client scope, including unchanged configuration.

#### Scenario: One client is absent

- **WHEN** a valid selected Route belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent Route remains usable.

#### Scenario: An external compatibility endpoint is absent

- **WHEN** no local compatibility service is installed
- **THEN** AIGW remains usable with any configured native HTTPS endpoint
- **AND** no external-gateway file, process, service, port, or lifecycle state
  is required.

#### Scenario: Cancellation precedes mutation admission

- **WHEN** setup, configuration selection, projection, or repair observes a
  cancelled request at its mutation boundary
- **THEN** it returns cancellation without starting that boundary's writes
- **AND** setup restores any Token already prepared before cancellation was
  observed, subject to the existing credential ownership guard.

#### Scenario: Preparation observes cancellation before persistence

- **WHEN** snapshot capture or projection preflight completes with the request
  cancelled
- **THEN** configuration persistence does not start
- **AND** existing configuration, backup, checkpoint and client files remain
  unchanged.

#### Scenario: Cancellation prevents a later client projection

- **WHEN** cancellation is observed before the next client adapter starts
- **THEN** that adapter performs no writes
- **AND** the registry compensates prior adapters in reverse order and the
  enclosing transaction compensates its configuration
- **AND** cancellation and any recovery conflicts remain inspectable errors.

#### Scenario: Cancellation follows the last admitted projection

- **WHEN** the last admitted adapter completes successfully after cancellation
  arrives during its synchronous operation
- **THEN** the completed transaction remains successful
- **AND** no post-completion cancellation check undoes its committed result.

#### Scenario: Identity migration is cancelled before writes

- **WHEN** Account renaming or verified finalization observes cancellation
  before credential preparation or finalization admission
- **THEN** configuration, backup and source and target credentials remain
  unchanged
- **AND** cancellation returns from the identity-migration service independently
  of the CLI; credential copies already admitted before a later configuration
  commit failure retain both slots for retry and rollback.

#### Scenario: Setup is requested for an existing installation

- **GIVEN** the current configuration already contains Routes
- **WHEN** interactive, explicit, manifest or internal setup is requested
- **THEN** the shared setup admission rejects it before prompting, Provider
  probing, client discovery or mutation
- **AND** configuration import, selection and Token rotation retain their
  separate operations rather than becoming implicit setup behavior.

#### Scenario: First-time setup finds an existing credential slot

- **GIVEN** the current configuration has no Routes but an Account Token is
  already available
- **WHEN** first-time setup is requested
- **THEN** that Token does not by itself classify the installation as configured
- **AND** setup may use or explicitly replace it with guarded compensation.

### Requirement: Extension preserves the control-plane core

An ordinary Provider SHALL be added through Account, endpoint, Route, Route,
and optional diagnostic declarations. A new client SHALL be added through one
admitted client Adapter and its conformance fixtures. Neither extension SHALL
introduce Provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add a client-native cloud model service

- **WHEN** an AWS model service exposes an admitted client protocol and the
  client already owns its credential chain and request signing
- **THEN** its Account and Routes use the ordinary manifest path
- **AND** the Route declares client-native authentication
- **AND** AIGW neither stores the cloud credential nor implements a signing
  Adapter.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing Provider data and client Adapters remain unchanged.

#### Scenario: Supply operational adapters in a different order

- **WHEN** the registry receives operational adapters in an order different
  from its admitted client declarations
- **THEN** discovery, planning and all-client application SHALL follow admission
  order, independently of constructor argument order
- **AND** a failed application SHALL compensate completed adapters in reverse
  application order.

#### Scenario: An adapter's compensation fails

- **WHEN** application fails after prior adapters have completed and one prior
  adapter reports a compensation failure
- **THEN** the registry SHALL still attempt every other prior adapter's
  compensation in reverse application order
- **AND** the returned error SHALL preserve the application failure and each
  compensation failure without claiming successful restoration.

### Requirement: Client deactivation is ownership-bounded

AIGW SHALL withdraw integration through the guarded projection transaction that
created it. Disable SHALL remove only that client's owned projection and obsolete
checkpoint; uninstall SHALL first disable all enabled Adapters and remove the
program only after withdrawal succeeds. Both SHALL preserve configuration,
Tokens, other Adapters, and neighboring user state. A retained previous
configuration MAY serve only as an explicit rollback source.

#### Scenario: Disable one client

- **WHEN** an operator disables one enabled client Adapter
- **THEN** only that client's AIGW-owned projection and ownership state are
  withdrawn
- **AND** the other client, capability configuration, credentials, and
  user-authored client settings remain unchanged
- **AND** the stale verified checkpoint is absent.

#### Scenario: A user extends settings created by AIGW

- **GIVEN** AIGW created a previously absent client settings file
- **AND** the user subsequently added fields outside AIGW's ownership
- **WHEN** the Adapter is disabled or the portable program is uninstalled
- **THEN** those user fields remain and only AIGW-owned fields are withdrawn
- **AND** the file is deleted only when withdrawal leaves it empty.

#### Scenario: Uninstall the portable program

- **WHEN** an operator uninstalls AIGW with one or more enabled client Adapters
- **THEN** every AIGW-owned client projection is withdrawn before the executable
  and its program rollback copy are removed
- **AND** capability configuration, credentials, neighboring user state, and the
  explicit previous-configuration backup remain available
- **AND** no verified checkpoint continues to claim the withdrawn projections.

### Requirement: Portable installation describes its own current files

`aigw installation` SHALL describe the invoked portable program from current
files without mutation, credential reads, valid Account configuration, or
executing a retained program. Versioned JSON SHALL bind absolute command and
payload paths, version, size, and SHA-256. A rollback copy SHALL use the same
identity model; absence SHALL be explicit. Unreadable or nonregular files SHALL
fail at their boundary. No persisted registry or checkout-only receipt SHALL
supply the result.

#### Scenario: Observe an installed program without source or configuration

- **GIVEN** a portable installation outside its source checkout
- **AND** Account configuration is absent or malformed
- **WHEN** the operator runs `aigw installation --json`
- **THEN** it reports the current program's file identity without modifying any
  file, reading credentials or starting a client
- **AND** a missing rollback copy is represented as `null`.

#### Scenario: Observe program replacement and rollback

- **WHEN** the operator observes an installation after update or rollback
- **THEN** the result reflects the files actually present at those paths
- **AND** install, update, rollback, uninstall and observation use one rollback
  path rule rather than parallel naming implementations.

### Requirement: Token rotation is credential-scoped

AIGW SHALL validate and replace only the selected Account's Token. Rotation
MUST leave Accounts, Models, Routes, Client Bindings, client configuration, ownership sidecars
and client-owned credentials unchanged. Setup, service creation and rotation SHALL use the same
Token replacement and compensation owner. Compensation SHALL inspect the
written Token, preserve a different observed value, and report incomplete
storage recovery.

#### Scenario: The next helper invocation observes rotation

- **WHEN** rotation successfully replaces the selected Account's Token
- **THEN** a matching projected helper reads the replacement on its next call
- **AND** rotation invokes no client and changes no client-owned credentials
- **AND** its result does not promise immediate refresh by a running client.

#### Scenario: A newer Token appears before compensation

- **WHEN** compensation observes a Token different from the one it wrote
- **THEN** it preserves the observed Token and reports the ownership conflict
- **AND** client-owned credentials remain unchanged.

#### Scenario: The operator cancels validation

- **WHEN** the rotation command is cancelled during Token validation
- **THEN** validation observes the command cancellation
- **AND** no replacement Token is persisted.

### Requirement: Service creation is an atomic client-scoped transition

Service creation SHALL admit a new Account and first Route before requesting
its Token. It SHALL select that Route and reconcile only its client's
projection through the shared synchronization transaction. An existing Account
or Route identity SHALL NOT be implicitly replaced.

#### Scenario: Add and select another service

- **WHEN** service creation succeeds for an installed admitted client
- **THEN** the stored Route and that client's configuration SHALL select the
  new service without requiring a second selection or synchronization command
- **AND** unrelated client configuration and ownership state SHALL be unchanged,
  including unrelated external edits.

#### Scenario: Creation admission fails

- **WHEN** the Account or Route already exists, desired configuration is
  invalid, or the request is cancelled before acquisition
- **THEN** creation SHALL reject the request before requesting a Token or
  changing configuration, credentials or client projections.

#### Scenario: Service creation cannot complete synchronization

- **WHEN** service creation stores its Token but configuration persistence or
  selected-client projection fails
- **THEN** it SHALL compensate through the shared Token replacement owner
- **AND** configuration and projection recovery SHALL use their existing owners
- **AND** it SHALL preserve the original failure and any compensation error
- **AND** failed compensation SHALL NOT be reported as restored storage.

### Requirement: Verification checkpoints remain bound to current configuration

The configuration Store SHALL own checkpoint admission, bounded locking,
guarded persistence, and compensation; live requests SHALL run outside the lock.
Verification SHALL create a checkpoint only while its configuration remains
current and SHALL NOT create or certify missing configuration. Reads SHALL
consume one complete JSON document. Creation and reading SHALL share one
nonempty, distinct admitted-client scope.

#### Scenario: Invalid checkpoint content or client scope

- **WHEN** a stored checkpoint includes an additional JSON value or trailing
  malformed content, or its scope is empty, repeated or unadmitted
- **THEN** reading SHALL fail rather than certify the valid prefix
- **AND** creation with invalid scope SHALL preserve configuration, backup and
  the previous checkpoint.

#### Scenario: Configuration changes before verification completes

- **WHEN** a live verification completes after configuration has changed
- **THEN** checkpoint admission fails with an instruction to verify again
- **AND** current configuration and any newer checkpoint remain unchanged.

#### Scenario: An external editor changes configuration during checkpoint commit

- **WHEN** the Store observes changed configuration after checkpoint persistence
- **THEN** it compensates only its unchanged checkpoint postimage
- **AND** it preserves an intervening checkpoint replacement and reports any
  incomplete compensation.

#### Scenario: Checkpoint admission is cancelled

- **WHEN** the command is cancelled while waiting for the mutation lock
- **THEN** checkpoint admission returns the cancellation without a checkpoint
- **AND** the other mutation keeps ownership of its lock.

#### Scenario: A verified recovery snapshot is requested

- **WHEN** a caller requests current configuration with its verified checkpoint
- **THEN** the configuration Store SHALL establish that their persisted meanings
  match before returning verified recovery state
- **AND** formatting-only differences SHALL preserve the exact captured bytes
- **AND** a mismatch SHALL fail without changing configuration, backup or
  checkpoint, without relying on a downstream caller to repeat the check.

### Requirement: Release helpers preserve selected source identity

Release metadata and artifact helpers SHALL use the selected source's origin
and repository. Ambient client defaults SHALL NOT select a different host,
protocol or API subfolder. Existing client credentials and unrelated settings
SHALL remain unchanged.

#### Scenario: GitLab configuration names another API endpoint

- **WHEN** the selected GitLab release source differs from ambient host aliases
  or stored API routing
- **THEN** metadata capture and artifact streaming SHALL reach the selected
  origin and its root API path
- **AND** the unrelated endpoint SHALL receive no request.

#### Scenario: GitHub CLI has an unrelated default host

- **WHEN** an admitted GitHub release uses the authenticated CLI fallback
- **THEN** metadata and downloads SHALL bind the selected source rather than
  the ambient `GH_HOST` value.

#### Scenario: Release coordinates have ambiguous URL meaning

- **WHEN** an origin lacks a hostname or carries a path, query or fragment,
  including an empty query or fragment marker
- **THEN** update and build admission SHALL reject it before network or
  helper execution
- **AND** repository coordinates SHALL retain their unescaped namespace and
  project meaning under native relative-path and URL encoding rules
- **AND** missing namespaces, dot segments or encoded separators SHALL NOT
  reach a Forge.

#### Scenario: A release uses an explicit IPv6 authority

- **WHEN** an HTTPS origin includes a bracketed IPv6 address, port and optional
  root slash with valid repository coordinates
- **THEN** admission SHALL preserve that source identity
- **AND** build metadata and runtime selection SHALL consume the same
  release-source address policy.

#### Scenario: A release embeds an explicit private-network peer

- **WHEN** a release configures an HTTP origin using a private, link-local or
  loopback IP address, `localhost`, or a reserved `.test` host
- **THEN** build metadata and runtime selection SHALL admit the same origin
  and repository without substituting an ambient peer
- **AND** public HTTP origins SHALL remain outside the admitted address set
- **AND** signature verification and explicit-HTTPS Token fallback SHALL retain
  their independent requirements; HTTP SHALL NOT imply transport confidentiality.

### Requirement: Local candidate identity is verified before a no-op result

An explicitly selected local portable candidate SHALL pass checksum and
target-layout admission even when its version equals the running version.
Equal version labels SHALL NOT substitute for equal program bytes. Admission
and comparison SHALL preserve the current executable and retained predecessor.

#### Scenario: Reapply the exact current program

- **WHEN** a verified same-version candidate contains the current executable's
  exact bytes
- **THEN** update reports an already-matching program without executing or
  replacing it, changing its predecessor, or consulting a remote source.

#### Scenario: Reapply an unverified or different same-version program

- **WHEN** the archive, checksum, target program, or current executable is
  unavailable or invalid, or the verified candidate's program bytes differ
- **THEN** update fails without reporting a verified no-op
- **AND** current and retained programs remain unchanged.

### Requirement: Repeated portable installation preserves rollback identity

Portable installation SHALL compare executable bytes before rotating the
predecessor. Identical reinstall SHALL preserve both program identities and MAY
restore executable permission; a fresh install SHALL create no predecessor.
Installation SHALL preserve shell configuration and `PATH`, identify the
installed executable, and explain direct-path invocation before assuming name
resolution.

#### Scenario: Install outside the command search path

- **GIVEN** the destination directory is absent from `PATH`
- **WHEN** portable installation succeeds
- **THEN** the result identifies the installed path and explains that `PATH`
  remains unchanged
- **AND** the operator can invoke that executable directly without changing
  shell configuration or relying on another installation.

#### Scenario: Reinstall an unchanged downloaded program

- **GIVEN** a portable executable is installed from another path
- **WHEN** the same executable bytes are installed again
- **THEN** the current program remains in place
- **AND** the predecessor retains its prior bytes or remains absent.

### Requirement: Projection-matched Account Token delivery

Every Account-Token Codex provider SHALL authenticate through the absolute AIGW
command. The helper SHALL carry the projected client, Account, and endpoint
fingerprint and compare it with the selected Route before reading a Token. AIGW
SHALL neither invoke native login nor change client credentials. Client-native
Routes SHALL project no AIGW Token helper; naming and catalogue selection
SHALL NOT alter credential ownership.

#### Scenario: Account-Token provider projection

- **WHEN** AIGW synchronizes a default or explicit Account-Token provider
- **THEN** the provider table declares `wire_api = "responses"`
- **AND** its auth command invokes the absolute AIGW executable with
  `credential codex <projection-fingerprint>`
- **AND** no Token value is copied into client files.

#### Scenario: Return to the default provider

- **WHEN** a Route changes from an explicit provider to the default provider
- **THEN** the old attributed provider table is removed transactionally
- **AND** the Route's declared authentication ownership is preserved.

#### Scenario: A retained projection no longer matches the selected Route

- **WHEN** a helper is invoked after its client, Account or endpoint differs
  from the selected Route
- **THEN** it returns a synchronization and reload instruction
- **AND** reads no Token and writes no credential bytes to standard output.

#### Scenario: Model or label changes without a credential-identity change

- **WHEN** only the selected model or label changes
- **THEN** the existing helper fingerprint still matches
- **AND** the helper reads the same Account's current Token.

#### Scenario: Multiple Codex homes receive the same Account-Token route

- **WHEN** setup discovers multiple admitted, AIGW-owned Codex targets
- **THEN** it projects the matching credential helper into each target
- **AND** reads or stores only the selected Account's Token
- **AND** leaves client-owned credentials unchanged.

### Requirement: Readiness observations remain separated from client execution

AIGW SHALL distinguish local configuration readiness, endpoint observations
and real-client verification. Status SHALL observe selection and owned
projections without reading Token values or invoking native clients. A present
Token, synchronized projection or successful endpoint probe SHALL NOT alone
certify that a real client can authenticate or perform inference.

#### Scenario: A synchronized projection has no live-client evidence

- **WHEN** a client has a valid AIGW-owned projection
- **THEN** status may report that projection as ready
- **AND** does not claim native authentication or inference has been proved.

#### Scenario: Real-client verification is requested

- **WHEN** the operator invokes `aigw verify --for <client>`
- **THEN** the admitted adapter invokes that selected native client
- **AND** reports evidence bounded to the request and client it observed.

### Requirement: Recovery execution and presentation are independent

Synchronization and repair SHALL select writes through `--dry-run` and output
format through `--json` independently. Successful preview and applied results
SHALL expose their execution mode and the same continuation as human output.
Rendering SHALL NOT own transaction commit or compensation.

#### Scenario: Preview a recovery operation as JSON

- **WHEN** `sync` or `repair` succeeds with `--dry-run --json`
- **THEN** output SHALL be one JSON document with `dry_run: true`
- **AND** `next_action` SHALL identify the corresponding apply command
- **AND** configuration, credentials and client projections SHALL remain
  unchanged.

#### Scenario: Apply a recovery operation with JSON output

- **WHEN** `sync` or `repair` succeeds with `--json` without `--dry-run`
- **THEN** the owned projection SHALL be applied
- **AND** output SHALL be one JSON document with `dry_run: false` and
  `next_action: "aigw check"`
- **AND** preview plans SHALL NOT be presented as observed per-target results.

#### Scenario: Output fails after a successful recovery commit

- **WHEN** a recovery operation commits and its result cannot be written
- **THEN** it SHALL return the output error and preserve the committed state
- **AND** generic presentation SHALL NOT claim that rollback occurred or will
  occur.

### Requirement: Invocation output has one owner

The CLI root SHALL finalize command output once, preserving command, output and
cleanup failures. Generic JSON errors SHALL use the same Problem and continuation
as human errors. Credential-helper stdout SHALL contain only credential output.

#### Scenario: A JSON-capable command fails before writing a result

- **WHEN** the selected command has parsed `--json` as true and fails before
  writing its result
- **THEN** stdout SHALL contain one JSON document with `ok: false`, `error` and
  `next_action`, preserving available evidence and impact
- **AND** the command SHALL return a nonzero exit status.

#### Scenario: Output format follows the parsed command

- **WHEN** `--json=false` is supplied, or `--json` appears after the `--`
  argument separator
- **THEN** the generic error SHALL use human output
- **AND** command-resolution failures before flag parsing SHALL use human output.

#### Scenario: Output cannot be completed

- **WHEN** the result writer fails or accepts only part of a write
- **THEN** the invocation SHALL preserve the write failure and any command error
- **AND** SHALL NOT append another result to the incomplete output
- **AND** a subsequent invocation SHALL start with independent output state.

#### Scenario: Credential helper fails

- **WHEN** the credential helper fails command validation or execution
- **THEN** the executable SHALL return a nonzero exit status without rendering
  a diagnostic on credential stdout
- **AND** stderr SHALL explain the failure and its recovery using safe,
  product-owned diagnostics, without echoing raw backend errors, configuration
  values or credential bytes, including failures during initialization
- **AND** stale projection rejection SHALL precede credential access and explain
  that the client must reload its synchronized configuration
- **AND** diagnostic write failures SHALL remain observable to the caller.

### Requirement: Command help preserves native metadata

Command help SHALL derive grammar, descriptions, examples and option metadata
from the command tree. pflag SHALL own flag notation, value types and declared
defaults. Help SHALL remain available without configuration or credential access.

#### Scenario: Inspect detailed command help

- **WHEN** a command declares detailed text, examples, typed flags, defaults or
  inherited options
- **THEN** help SHALL expose those declarations in the existing terminal layout
- **AND** defaults SHALL reflect declarations rather than previously parsed values
- **AND** narrow output SHALL preserve the same option meaning.

#### Scenario: Inspect any command before setup

- **WHEN** any command is invoked with `--help` before configuration exists
- **THEN** help SHALL show its native option metadata without creating
  configuration storage or invoking its operation.

### Requirement: Argument admission precedes mutation

The CLI SHALL validate positional arguments, required flags, flag relationships
and command-specific argument shape before acquiring a configuration mutation
lock or executing an operation. Cobra SHALL own shared flag validation.

#### Scenario: Select a local update with invalid paths or options

- **WHEN** an explicitly supplied candidate or checksum path is empty or
  whitespace-only, its paired option is missing, or rollback is combined with
  candidate options
- **THEN** the command SHALL reject the invocation before creating configuration
  storage or calling an online update, candidate update or rollback operation.

#### Scenario: Supply invalid manifest setup arguments

- **WHEN** the manifest path or explicitly selected Account ID is blank,
  manifest mode is combined with single-route options, JSON output is
  requested without manifest mode, or manifest-mode Token input has no Account
  owner
- **THEN** setup SHALL reject the invocation before creating configuration
  storage, reading the manifest, or executing setup.

#### Scenario: Supply incomplete account finalization intent

- **WHEN** account finalization lacks explicit old/new identifiers, or credential
  rotation confirmation is requested without finalization
- **THEN** argument admission SHALL reject the invocation before acquiring a
  configuration lock or creating configuration storage
- **AND** the diagnostic SHALL direct the operator to that command's help,
  rather than an unrelated readiness check.

#### Scenario: Supply invalid rename arguments

- **WHEN** noninteractive Account or Route rename omits an identifier, an
  explicitly supplied identifier violates the configuration identifier contract,
  or an incomplete interactive invocation has no prompt capability
- **THEN** argument admission SHALL reject it before configuration access,
  mutation-lock acquisition, credential access or discovery
- **AND** the diagnostic SHALL direct the operator to the invoked command's help
- **AND** complete explicit identifiers SHALL require no prompt, while guided
  rename SHALL remain available when interactive input can complete the request.

#### Scenario: Supply invalid selection or adapter arguments

- **WHEN** noninteractive selection omits its Route, an adapter client is not
  admitted, its executable is missing or blank, or a required target is missing
  or an explicitly supplied target is blank
- **THEN** the command SHALL reject the invocation with an actionable repair
  command before creating configuration storage
- **AND** it SHALL NOT invoke credential access, client discovery or mutation.

#### Scenario: Supply invalid creation or import arguments

- **WHEN** service or Route creation supplies an invalid identifier, omits
  required client/model/Account flags, supplies blank required values, selects
  an inadmitted client, or configuration import supplies a blank manifest path
- **THEN** the command SHALL reject the invocation with actionable guidance
  before creating configuration storage or accessing credentials.

#### Scenario: Supply invalid metadata edit intent

- **WHEN** Account or Route editing supplies no editable flag, an invalid
  identifier or a blank label or endpoint, Route removal supplies an invalid
  identifier, or platform-credential connection lacks an interactive terminal
- **THEN** admission SHALL reject the invocation before configuration access or
  creation, mutation-lock acquisition or credential access
- **AND** native flag-group failures SHALL identify the invoked command's help,
  not an unrelated readiness check
- **AND** explicit empty Route purpose SHALL remain a valid request to clear
  that optional display field.

### Requirement: Program replacement has an explicit client reconciliation boundary

Update and rollback SHALL activate program bytes without silently changing
client settings or schema, then direct reconciliation through `aigw sync`.
Rollback SHALL first qualify the exact retained program against an isolated
configuration copy without credentials or client discovery. Failure SHALL
preserve both programs and operator configuration. Compatibility restoration
SHALL be explicit; release qualification SHALL use the real predecessor and
retained client state.

#### Scenario: The predecessor cannot read the current configuration

- **WHEN** the current configuration contains a capability unknown to the
  retained program
- **THEN** program rollback fails before replacing either program file
- **AND** the error identifies configuration compatibility rather than claiming
  that the previous program became active
- **AND** after an explicit compatible configuration restoration, rollback can
  activate the predecessor and that predecessor can export the configuration.

#### Scenario: A replacement changes the credential helper contract

- **WHEN** an operator activates another program version
- **THEN** the active program's explicit synchronization SHALL reconcile its
  AIGW-owned client projection before the operator resumes client work
- **AND** lifecycle acceptance SHALL execute the projected helper rather than
  infer its arguments from the candidate implementation.

#### Scenario: A supported release pair retains enabled clients

- **WHEN** a configured client remains enabled through upgrade, rollback and
  re-upgrade between a qualified predecessor and successor
- **THEN** each program replacement SHALL preserve the AIGW configuration bytes,
  including explicit client locations and selected Routes
- **AND** real-client acceptance SHALL check the exact active executable and
  complete a request through that retained integration after each transition.

### Requirement: Online update owns its temporary resources and preserves failure causes

An online update SHALL own all peer download directories within one operation
workspace and attempt to remove that workspace on every return. Cleanup SHALL
preserve unrelated files and SHALL NOT alter the completed installation outcome.
Reported failures SHALL retain their original causes for caller inspection.

#### Scenario: A release peer is temporarily unavailable

- **WHEN** a configured peer fails during metadata or asset transport
- **THEN** the updater SHALL apply the same unavailability classification for
  GitLab and GitHub and continue with another admitted peer
- **AND** if every peer is unavailable, the error SHALL retain every peer's cause
- **AND** authentication or integrity failures SHALL remain terminal.

#### Scenario: Workspace cleanup fails

- **WHEN** the operation's workspace cannot be fully removed
- **THEN** the error SHALL identify that exact workspace and preserve the cleanup
  cause together with any preceding failure
- **AND** if replacement completed, the diagnostic SHALL state that the program
  was updated rather than implying that installation was unchanged or rolled back.

### Requirement: Release construction owns only its operation resources

Release work SHALL own its workspace, private replacement backup, processes,
and cleanup. Commands SHALL preserve failure causes, directories, environments,
parent output handles, bounded captures, and complete streamed channels.
Interruption SHALL propagate through every stage; return SHALL terminate only
owned process-group or Job descendants. Signal interception SHALL be scoped to
active non-interactive work and restore prior behavior.

#### Scenario: Existing output has an unrelated neighboring file

- **WHEN** a release replaces an existing output directory
- **THEN** files outside that directory and the operation's private workspace
  SHALL remain unchanged, regardless of their names
- **AND** failed publication SHALL restore the prior output; if restoration
  also fails, the error SHALL retain both causes and identify the preserved backup.

#### Scenario: Output was published but predecessor cleanup failed

- **WHEN** the new output is installed and removal of its private backup fails
- **THEN** the result SHALL report completed publication and the cleanup error
- **AND** the retained predecessor SHALL remain discoverable at the reported path.

#### Scenario: A build tool leaves a background descendant

- **WHEN** its direct process exits or the release command is interrupted
- **THEN** the process owner SHALL terminate descendants still in its native
  process group or Job and preserve unrelated processes
- **AND** cancellation, process exit and cleanup failures SHALL remain
  distinguishable to callers.

#### Scenario: A tool emits a large build log

- **WHEN** a release tool streams output larger than the captured-result budget
- **THEN** both output channels SHALL reach their caller-owned destinations
- **AND** the child SHALL receive the selected directory, environment and closed
  standard input rather than ambient interactive shell state.

#### Scenario: The host interrupts an isolated child invocation

- **WHEN** an interrupt or termination signal arrives while an owned child runs
- **THEN** its caller SHALL receive cancellation after owned-process cleanup
- **AND** returning from the invocation SHALL restore the previous host signal
  behavior rather than retaining a handler during later interactive input.

### Requirement: Peer ancestry observation preserves local reference state

Publication ancestry checks SHALL consume the exact observed peer commit and
SHALL preserve existing Git references and `FETCH_HEAD`. An already available
commit requires no transport; fetching a missing object SHALL create no
temporary branch or observation reference.

#### Scenario: Compare an observed peer commit with the publication source

- **WHEN** both commits are available
- **THEN** the comparison SHALL distinguish ancestry from genuine divergence
- **AND** transport or Git execution failures SHALL remain errors rather than
  being interpreted as authorization to force a divergent update.

### Requirement: Catalogue observation owns an isolated and bounded probe lifetime

Bundled and effective catalogue readers SHALL execute with an operation-owned
client home and bounded subprocess lifetime. They SHALL preserve the operator's
configuration and report cleanup failures with the exact owned path and original
failure causes. Repository verification SHALL apply the same ownership rule to
its generated catalogue input.

#### Scenario: Client catalogue retrieval fails after identifying the executable

- **WHEN** the client reports its version but catalogue retrieval fails
- **THEN** the bundled reader SHALL retain the measured executable identity and
  original command failure
- **AND** successful cleanup SHALL remove the isolated home; failed cleanup
  SHALL add its cause without discarding the command failure or measured identity.

### Requirement: Failure recovery retains owned-resource cleanup outcomes

Credential writes, explicit client verification and coverage execution SHALL
retain their original failure when resource cleanup also fails. Cleanup SHALL
name the exact owned resource and SHALL NOT silently report success.

#### Scenario: A credential replacement fails before publication

- **WHEN** replacement cannot rename its staging file and staging removal also fails
- **THEN** the original credential SHALL remain unchanged and both failure causes
  SHALL be observable with the exact retained staging name
- **AND** once replacement commits, cleanup SHALL NOT remove a subsequently
  recreated file at the old staging name.

#### Scenario: Client verification creates additional output

- **WHEN** a client verification command writes into its working directory
- **THEN** that directory SHALL be private to the verification operation and
  reclaimed on success or failure
- **AND** cleanup failure SHALL retain the invocation error and exact workspace.

#### Scenario: Coverage measurement succeeds but route cleanup fails

- **WHEN** a coverage command cannot remove its owned temporary route
- **THEN** its exit status SHALL indicate failure and its diagnostic SHALL name
  that file without suppressing any preceding test failure.

#### Scenario: A guarded file write fails during staging

- **WHEN** writing, setting permissions or syncing an owned temporary file fails
- **THEN** the commit helper SHALL close its handle exactly once and retain any
  close failure alongside the original error
- **AND** the writer SHALL remove only its failed staging file and report any
  cleanup failure without modifying unrelated files
- **AND** after a successful rename, it SHALL relinquish the old staging name.
