## ADDED Requirements

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
source gates, and a reproducible installable matrix. Hosted CI and delivery
SHALL consume, not gate, that accepted result. Change tasks SHALL close
implementation and candidate acceptance before archive; canonical specs and
design SHALL retain publication, installed-asset and lane-retirement duties
until proved.

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
- **AND** accepted-ref and tag CI, published assets, installed lifecycle, and
  lane retirement SHALL each require their own post-archive evidence
- **AND** archive SHALL NOT claim those outcomes or block local acceptance when
  a peer is unavailable.

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

### Requirement: Pre-tag artifact acceptance binds to signed source

Before a stable tag exists, AIGW SHALL accept an explicitly selected artifact
candidate only when its trusted signature and canonical provenance match the
current signed source commit and locked inputs. Candidate mode SHALL require a
clean checkout and reject a selected or same-version local release tag. Release
verification and publication SHALL still require a signed tag. Supplied
artifact bytes SHALL not be rebuilt or replaced.

#### Scenario: Signed candidate precedes its release tag

- **WHEN** an operator selects candidate acceptance for a signed artifact
  matrix before the stable tag exists
- **THEN** AIGW verifies the artifact signer, signed HEAD, provenance, and
  supplied native bytes before running client and lifecycle acceptance.

#### Scenario: Candidate mode could bypass a release tag

- **WHEN** a release tag is selected or a same-version local tag exists
- **THEN** candidate acceptance is rejected
- **AND** release verification and publication still require tag-signature
  validation.

### Requirement: Native verification runs the declared Mise release

Local verification and every Forge job SHALL run the Mise release that the
repository declares. A Mise release whose diagnostics the repository's tests
assert SHALL become the CUE pin and the repository floor together, and a
runner-owned Mise SHALL be replaced before a job consumes that declaration.

#### Scenario: A Mise release changes an asserted diagnostic

- **WHEN** a repository test asserts a diagnostic that only a newer Mise release
  prints
- **THEN** the CUE pin, the generated Forge projections and the repository Mise
  floor SHALL name that release
- **AND** the hosted native jobs SHALL pass with it before integration.

#### Scenario: A runner owns its Mise

- **WHEN** the declared Mise advances while a runner owns its Mise executable,
  such as GitLab's Windows ARM64 runner
- **THEN** that runner's executable and shim SHALL be replaced with the verified
  release before any of its jobs consumes the declaration.

### Requirement: Release SBOM covers the native binary matrix

The release builder SHALL catalog every emitted native executable through the
locked Syft Go-binary and file catalogers. It SHALL retain platform-specific
dependencies and file digests in one SPDX document without a custom SBOM merger.

#### Scenario: Native release evidence is constructed

- **WHEN** the portable release matrix is built
- **THEN** native conformance SHALL compare every SBOM binary path and SHA-256
  with the emitted executable matrix
- **AND** a missing or extra binary count SHALL fail before release signing
- **AND** a single selected binary or a scan of compressed archives SHALL NOT
  establish complete platform coverage
- **AND** full-lock license and vulnerability evidence SHALL remain distinct
  from the binary runtime inventory.

### Requirement: Native release targets are independent of the build host

macOS AMD64 and ARM64 artifacts SHALL retain the declared macOS 13 deployment
floor supported by the locked Go toolchain and selected native APIs. A newer
build host or SDK SHALL NOT silently raise that floor. Native artifact evidence
SHALL inspect the linked executable, not infer compatibility from configuration
or successful execution on the build host.

#### Scenario: A newer SDK links the native credential implementation

- **WHEN** macOS construction uses external linking for the native credential API
- **THEN** both architecture archives SHALL declare macOS 13.0 as their minimum
- **AND** cached CGO objects compiled for a newer target SHALL NOT cause deployment warnings
- **AND** native signature and deterministic archive validation SHALL remain required
- **AND** linked-header acceptance SHALL NOT imply execution on an older OS.

#### Scenario: Native CGO cache contains a newer deployment target

- **WHEN** a private empty cache is warmed at a newer deployment target
- **THEN** the matching host architecture SHALL rebuild through the declared native release configuration
- **AND** its actual CPU, deployment floor and signature SHALL be verified without deployment warnings
- **AND** warmup and rebuild SHALL retain one original bounded deadline and no operator signing or Forge credentials
- **AND** the independent full-matrix journey SHALL retain both architecture archives and deterministic signatures.

### Requirement: Native artifact acceptance has its own execution closure

Selected prebuilt or tagged products SHALL use only the artifact journey's
required native tools unless full source quality or lock refresh is selected.
Source, review, and full qualification SHALL retain the complete locked
bootstrap, quality, and security graph. A same-version verifier MAY accept an
exact signed untagged product commit with matrix signatures, clean source,
and matching provenance. Artifact results SHALL NOT replace failed source
qualification.

#### Scenario: A newer verifier consumes a prebuilt candidate

- **WHEN** native acceptance selects existing candidate bytes and their exact
  signed commit without full quality or lock refresh
- **THEN** the original Runner identity SHALL execute the artifact journey
  without npm bootstrap or repeated source-wide tests
- **AND** mutable refs, unsigned objects and mismatched provenance SHALL fail
  before product execution.

#### Scenario: An existing transport release supplies a later candidate

- **WHEN** an existing signed transport Release holds an exact-source packet
- **THEN** native acquisition SHALL select `public-inputs-<input-sha256>.tar`
  without replacing the carrier's original assets or creating a new Release
- **AND** the outer checksum, complete matrix signatures, exact candidate
  provenance and published predecessor SHALL determine product admission
- **AND** the signed carrier's named target SHALL remain verified, but its
  history SHALL NOT substitute for or restrict candidate provenance
- **AND** one bounded acquisition SHALL reclaim its private workspace on success,
  failure and cancellation.

#### Scenario: Source qualification remains required

- **WHEN** full quality, lock refresh or a source-only native journey is selected
- **THEN** the complete declared source tool and gate graph SHALL remain required
- **AND** a successful prebuilt journey SHALL NOT close an outstanding quality
  or release-security failure.

#### Scenario: Manual diagnostics cannot impersonate required checks

- **WHEN** GitHub dispatches a manual workflow, including tag or single-platform
  diagnostics
- **THEN** its checks SHALL have identities distinct from canonical required
  review and push checks, even when skipped jobs report success
- **AND** release admission SHALL retain the exact tag push and canonical job set
- **AND** GitLab SHALL retain its complete merge-request jobs and native
  merge-request pipeline admission, independently of manual native results.
