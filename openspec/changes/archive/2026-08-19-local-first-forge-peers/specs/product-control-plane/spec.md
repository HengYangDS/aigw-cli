## MODIFIED Requirements

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

## REMOVED Requirements

### Requirement: Semantic Forge history projection

**Reason**: Reconstructing provider-specific commits creates multiple product
histories and violates exact local object authority.

**Migration**: Verify and publish the unchanged local signed commit with an
explicit product trust anchor and peer transport credential.

### Requirement: Local release-root promotion

**Reason**: Local branch-role transitions are governed by ETHOS; a second
repository-owned transition command is a parallel lifecycle owner.

**Migration**: Use the governed ETHOS local lifecycle, then pass the resulting
accepted `main` to the AIGW peer projector.
