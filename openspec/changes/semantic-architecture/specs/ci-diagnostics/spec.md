## ADDED Requirements

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

### Requirement: Portable exact-version CI bootstrap

GitLab Linux bootstrap SHALL use the exact Mise image version and digest
projected by CUE and the repository-locked tool closure. Native distribution
clients SHALL own bounded transport failure handling. Linux jobs SHALL inherit
this owner rather than duplicate installation, substitute an image or installer,
or weaken integrity policy.

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

### Requirement: Release transport is independent of CI execution

The existing release publisher SHALL accept local operator execution and CI
execution through the same signed-artifact and source-verification path.
GitLab authentication SHALL select exactly one access token or CI job token
without manufacturing a job identity or exporting a signing private key.

#### Scenario: A local operator publishes to GitLab

- **WHEN** `GITLAB_TOKEN` is supplied and `CI_JOB_TOKEN` is absent
- **THEN** the existing upload and publication commands SHALL use the native
  access-token header for upload, metadata and same-origin asset readback
- **AND** CI job-token execution SHALL retain its native job-token header
- **AND** ambiguous or missing credentials SHALL fail before network access
- **AND** redirects away from the selected authority SHALL strip credentials,
  including when the redirect chain later returns to that authority
- **AND** artifact trust, exact tagged source and complete asset verification
  SHALL remain unchanged.

#### Scenario: Published artifacts are qualified independently

- **WHEN** the operator publishes one accepted, signed artifact matrix to
  selected peers and requests post-publication verification
- **THEN** each peer SHALL download only its own complete artifact set and run
  the same read-only signature, inventory and tagged-provenance verifier
- **AND** hosted verification SHALL require public trust and download access,
  not a signing private key or a new artifact build
- **AND** tag creation SHALL still run source and native-platform checks
- **AND** missing trust, missing assets, checksum mismatch, invalid signatures
  or mismatched source SHALL fail rather than trigger reconstruction
- **AND** source CI, artifact verification and installed-product acceptance
  SHALL retain distinct completion claims.

### Requirement: Diagnostic assertions read text, not terminal presentation

Tests and gates that assert a native tool's diagnostics SHALL judge the
diagnostic text after removing terminal escape sequences. A hosted runner that
colors the output SHALL reach the same judgement as an uncolored local
terminal.

#### Scenario: A hosted runner colors a native tool's warning

- **WHEN** a native tool prints an expected warning with ANSI escape sequences
  between its words
- **THEN** the assertion SHALL count that warning exactly once
- **AND** the same output without escape sequences SHALL yield the same count.

### Requirement: Artifact jobs prepare only their peer's download tool

A native artifact job SHALL prepare the Forge CLI of its own peer and not the
other peer's. Source and full-quality jobs MAY prepare both.

#### Scenario: A GitHub retained-store run uses peer tool sources

- **WHEN** a GitHub artifact job runs with `tool_source=peer`
- **THEN** it SHALL NOT request glab from any carrier.
