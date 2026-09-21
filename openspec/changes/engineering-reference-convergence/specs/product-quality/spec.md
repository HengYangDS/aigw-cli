# Spec Delta

## ADDED Requirements

### Requirement: Engineering-reference quality is demonstrated by behavior

AIGW SHALL qualify as an engineering reference only when its necessary product behavior is expressed through cohesive owners, narrow interfaces, deterministic configuration, observable failure semantics, and tests capable of disproving the design. Additional abstractions, frameworks, rules, documents, or generated artifacts SHALL NOT count as quality unless they remove greater accidental complexity or protect a named risk.

#### Scenario: An implementation is proposed as a reference pattern

- **WHEN** a maintainer evaluates a product path for reference quality
- **THEN** the path SHALL demonstrate its invariant through public behavior, focused adversarial tests, and a reproducible contributor journey
- **AND** every retained abstraction and dependency SHALL identify the responsibility or maintenance cost it removes.

#### Scenario: A simpler complete design exists

- **WHEN** two implementations satisfy the same supported behavior and evidence obligations
- **THEN** AIGW SHALL retain the design with fewer authorities, states, dependencies, and failure modes
- **AND** delete the superseded implementation and its unconsumed tests, configuration, and documentation.

### Requirement: Quality constraints are comprehensive and proportionate

The repository quality graph SHALL cover every tracked source, test, configuration, documentation, schema, workflow, and generated projection with the mature native tool for each concern where it provides net value. Numeric limits SHALL be derived from observed risk and reviewed distributions; a tighter number SHALL be adopted only when it improves maintainability without fragmenting coherent logic or encouraging cosmetic restructuring.

#### Scenario: A quality threshold is tightened

- **WHEN** a maintainer proposes a lower size, complexity, nesting, parameter, coverage, or performance threshold
- **THEN** the proposal SHALL include current distribution, affected semantic owners, false-positive cost, and remediation path
- **AND** the accepted limit SHALL preserve coherent domain expression.

#### Scenario: A tracked format has no effective gate

- **WHEN** repository inventory finds a current format or generated projection outside the executable quality graph
- **THEN** the existing quality authority SHALL add the appropriate mature validator or explicitly remove the unsupported carrier
- **AND** a hand-written duplicate checker SHALL not be introduced when a maintained tool supplies the contract.

### Requirement: Delivery completion is evidence-bound

Quality completion SHALL require separate proof of local gates, exact source,
hosted CI, peer publication, Git identity, artifact integrity, installation,
runtime acceptance, and housekeeping. Release completion SHALL additionally bind
one signed tag to immutable assets, checksums, native Release records, and
supported-platform results. Each peer verifies its own objects and assets; the
admitted aggregate executor set MAY supply native platform evidence without
duplicating unavailable runners.

#### Scenario: Both publication planes complete

- **WHEN** both selected publication planes satisfy their Forge-object and
  distribution-byte contracts for one accepted release
- **THEN** the aggregate publication stage SHALL be complete.

#### Scenario: Local proof passes but delivery is incomplete

- **WHEN** hosted CI, a selected peer, exact object identity, asset integrity,
  installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT
  claim terminal completion.

#### Scenario: Terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted product object
  and obsolete lanes, policies, compatibility paths, temporary assets, and
  stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each
  independent boundary.

## MODIFIED Requirements

### Requirement: Independent Forge parity

GitLab and GitHub SHALL be independent projections of one local Git object
authority. For every newly published product branch and formal release, local
Git and each selected peer SHALL expose the exact same commit OID, annotated tag
object OID, peeled commit, and tree. Tree-only equality, provider-qualified tag
namespaces, identity replay, and commit maps SHALL NOT be accepted as parity.

#### Scenario: Equivalent provider projection

- **WHEN** one signed local commit or annotated tag is published to both peers
- **THEN** the complete object identity SHALL be exactly equal on local Git,
  GitLab, and GitHub.

#### Scenario: Real source drift

- **WHEN** a peer commit or tag has an equal tree but a different object OID
- **THEN** synchronization SHALL fail as real product-object drift.

### Requirement: Forge capability projection

One evidence graph and CI topology SHALL separate product proof from Forge
capacity. Each projection includes only runnable native jobs; aggregate proof
retains every supported platform. Manual qualification MAY select one platform;
omitted or `all` keeps the available set. Review, accepted-branch, and tag events
keep their required source and native set. Missing capacity stays explicit. No
partial, cross-built, optional, pending, or `allow_failure` result SHALL count as
native or release proof.

#### Scenario: one Forge lacks a Windows executor

- **WHEN** another independent publication plane supplies admitted native Windows evidence
- **THEN** the Forge without Windows capacity SHALL omit its Windows job
- **AND** the product evidence model SHALL continue to require Windows
- **AND** cross-compilation SHALL NOT be reported as native evidence.

#### Scenario: Windows capacity is admitted later

- **WHEN** GitLab gains a qualified Windows executor
- **THEN** one capability declaration SHALL restore the generated native Windows job
- **AND** no parallel workflow or compatibility switch SHALL be introduced.

#### Scenario: A maintainer qualifies an updated Windows toolchain

- **WHEN** manual verification explicitly selects Windows and full quality
- **THEN** the Windows native job SHALL run its existing complete quality,
  source and packaged lifecycle commands without launching unrelated native jobs
- **AND** source quality SHALL remain selected and other required platforms
  SHALL retain their independent evidence obligations.

#### Scenario: A platform selector is supplied during release or review admission

- **WHEN** a tag, review, or accepted-branch push triggers verification
- **THEN** its complete available native set SHALL remain selected
- **AND** manual qualification SHALL NOT replace missing review checks.

### Requirement: Portable exact-version CI bootstrap

GitLab Linux bootstrap SHALL use the exact Mise image version and digest
projected by CUE. It SHALL install the minimal operating-system capabilities and
repository-locked tools required by the graph, including CGO when declared.
Every Linux job SHALL inherit this owner; no job may duplicate installation,
substitute another image or installer, or weaken integrity or transport policy.

#### Scenario: The repository graph needs a Linux host capability

- **WHEN** a locked tool, trust check, native compiler path, or race-enabled test
  needs an operating-system capability absent from the selected Linux image
- **THEN** the CUE-owned Linux toolchain SHALL declare and install the minimal
  complete package set before invoking Mise or the repository graph
- **AND** all GitLab Linux jobs SHALL inherit the same preparation without
  job-local copies, alternate images, or unbounded retries
- **AND** jobs that exercise CGO-dependent behavior SHALL explicitly enable CGO
  through the same CUE projection.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the native distribution client may retry within its bounded policy
- **AND** unresolved transport or integrity failure stops the job without
  claiming bootstrap success or substituting an unverified tool
- **AND** a retry retains the selected lock and integrity requirements.

## REMOVED Requirements

### Requirement: complete delivery evidence

**Reason:** The requirement combined aggregate delivery, Forge capacity,
distribution trust, credential continuity, and real credential-store
qualification under one quality owner.

**Migration:** Aggregate completion and Forge evidence survive in
`Delivery completion is evidence-bound`, `Independent Forge parity`, and
`Forge capability projection`. Distribution trust and final bytes move to
`release-distribution`. Credential continuity and real-store qualification move
to `secret-storage`.
