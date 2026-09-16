## MODIFIED Requirements

### Requirement: Source acceptance precedes delivery completion

Source acceptance SHALL require valid official Change artifacts, exact-source
quality evidence and authorized object-preserving integration. An active
Change MAY accompany that source into accepted or release refs while external
delivery remains incomplete. Its original tasks SHALL retain the remaining
work and be updated only after the corresponding outcomes are observed.
Archive SHALL follow completed Change obligations, not become a prerequisite
for the integration that enables them. AIGW SHALL retain native OpenSpec
validation without a second branch-based lifecycle checker.

#### Scenario: Active Change reaches source verification

- **WHEN** source verification observes an active Change in a work lane,
  proposal, accepted branch or release branch
- **THEN** the same official artifact validation and product quality graph run
- **AND** the presence of its task carrier alone SHALL NOT reject valid source
- **AND** malformed artifacts or failed quality checks still block acceptance
- **AND** source acceptance SHALL NOT mark pending publication, installation or
  cleanup complete.

### Requirement: Portable exact-version CI bootstrap

GitLab Linux bootstrap SHALL consume the exact mise image version and digest
from the CUE authority and install the declared tool closure from repository
locks. Native distribution clients SHALL own transport and bounded failure
handling; the repository SHALL NOT retain a second installer, force an
incidental HTTP version, or invent a mirror-package requirement.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the native distribution client may retry within its bounded policy
- **AND** unresolved transport or integrity failure stops the job without
  claiming bootstrap success or substituting an unverified tool
- **AND** a retry retains the selected lock and integrity requirements.

### Requirement: faithful quantitative quality evidence

Go statement coverage SHALL be measured under one machine policy owning the
aggregate floor, package observation, comparison, risk, remediation, and
review. Every canonical production package MUST remain visible. Packages with
measurable statements MUST execute owned statements and retain exact ratios;
proven declaration-only and native zero-statement packages MUST report not
applicable instead of an invented ratio. Evidence MUST bind raw counts, package,
revision and tree, toolchain, and policy digest. The repository SHALL NOT infer
branch coverage from statement data or depend on an unavailable analyzer to
manufacture a stronger-looking claim.

#### Scenario: quantitative evidence is evaluated

- **WHEN** coverage is admitted for promotion
- **THEN** aggregate statement evidence SHALL be strictly greater than 95
  percent
- **AND** every canonical production package SHALL be present in the same
  complete evidence set
- **AND** every package with measurable statements SHALL remain executed and
  report its exact statement ratio
- **AND** the verdict SHALL be independent of duplicated literals or inferred
  metrics.

#### Scenario: a quantitative boundary or observation contract is not met

- **WHEN** the aggregate ratio is equal to or below the canonical floor, or a
  package with measurable statements is absent or wholly unexecuted, or the
  observation is contradictory or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before
  promotion.

#### Scenario: a package has no statement denominator

- **WHEN** native Go evidence contains zero measured statements, or the package
  has no counters and Go-selected source proves it has no function bodies
- **THEN** the package SHALL remain visible with coverage not applicable
- **AND** it SHALL add neither fabricated statements nor a percentage to the
  aggregate
- **AND** missing, unreadable or mismatched source evidence SHALL fail rather
  than silently exempt a package.

#### Scenario: an unsupported coverage metric is proposed

- **WHEN** the locked language toolchain cannot produce a claimed metric and no
  maintained admitted analyzer owns it
- **THEN** the repository SHALL omit that claim rather than infer, relabel, or
  preserve it through an abandoned dependency.

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid.

#### Scenario: a package owns no branches

- **WHEN** a canonical package contains statements but no independently
  measurable branch decisions
- **THEN** the package SHALL remain visible through its exact statement ratio
- **AND** the repository SHALL NOT invent a 100-percent branch ratio.

#### Scenario: aggregate coverage carries the quantitative veto

- **WHEN** a package has a small or volatile denominator while the aggregate
  floor passes
- **THEN** the exact package ratio SHALL remain visible for review
- **AND** the package ratio SHALL NOT independently veto an otherwise valid
  aggregate result.

### Requirement: one complete quality graph

The repository SHALL expose one quality graph reused by local development,
exact-HEAD proof, GitLab, and GitHub. One declarative topology SHALL generate
Forge files; generated files MUST NOT own policy, and repository commands SHALL
own behavior. Projection drift fails first. Product targets, release assets,
native acceptance, and host compatibility are distinct claims;
cross-compilation proves only artifacts. A Forge SHALL schedule one verification graph for each admitted event. Separate
review, accepted-branch and release-branch events remain separate evidence
stages until an admitted verifier can prove safe cross-event reuse.

#### Scenario: a new repository owner is added

- **WHEN** tracked material is added or changed
- **THEN** its semantic class SHALL select every applicable quality check without an exclusion.

#### Scenario: a new package or test owner is added

- **WHEN** tracked Go source changes
- **THEN** architecture, static analysis, formatting, coverage, governance, and cross-platform contracts SHALL evaluate the new owner without an exclusion list.

#### Scenario: a projection diverges

- **WHEN** a tracked Forge file differs from the deterministic projection
- **THEN** source verification SHALL fail with the exact file before expensive tests.

#### Scenario: A required native runner is unavailable

- **WHEN** a Forge lacks an admitted executor for a supported platform
- **THEN** its projection SHALL omit that executor explicitly while aggregate
  product evidence still requires the platform
- **AND** exact-commit native evidence from another admitted executor MAY satisfy
  that platform fact; another operating system or cross-compile SHALL NOT.

#### Scenario: a release asset is cross-compiled

- **WHEN** CI produces an archive for an OS and architecture not represented by that runner
- **THEN** the archive MAY satisfy the release-asset matrix
- **AND** it SHALL NOT be reported as native acceptance or developer-host proof.

#### Scenario: a projection is regenerated

- **WHEN** the CI authority is rendered
- **THEN** every Forge-native file SHALL be produced deterministically
- **AND** no separate parser or duplicated policy SHALL be required.

#### Scenario: A GitLab job uses a toolchain container

- **WHEN** the runner prepares a container-backed verification job
- **THEN** the projected image configuration yields control to the runner shell
- **AND** the image's own entrypoint cannot reinterpret runner shell arguments

#### Scenario: Tests run inside a Forge job

- **WHEN** a test verifies the generic source gate sequence
- **THEN** it is independent of inherited Forge provenance variables
- **AND** dedicated provenance tests supply their own complete inputs

#### Scenario: A required native runner is misconfigured

- **WHEN** its operating-system shell or locked toolchain cannot start
- **THEN** the native gate fails explicitly
- **AND** no cross-build or different operating system is reported as a substitute

#### Scenario: a developer submits a proposal for review

- **WHEN** a proposal targets `dev` through a pull request or merge request
- **THEN** the review head SHA SHALL receive the complete verification graph
- **AND** the proposal branch push SHALL NOT start a parallel copy of that graph.

#### Scenario: a maintainer publishes an accepted product object

- **WHEN** local accepted `main` is projected unchanged to peer `main` and `dev`
- **THEN** the peer SHALL verify each resulting accepted-branch and release-branch
  event for its exact object
- **AND** only the release-branch push SHALL require accepted-ref parity; equal
  object IDs alone SHALL NOT suppress the accepted-branch graph.

#### Scenario: explicit diagnosis is required

- **WHEN** a maintainer explicitly dispatches verification
- **THEN** the selected Forge SHALL run the complete graph for the selected ref.

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent peer publication, exact
branch and tag identity, asset integrity, installation, runtime acceptance, and
repository housekeeping. A release SHALL be complete only when its one signed
tag object, immutable assets, checksums, peer-native Release records, and
supported-platform acceptance are verified at their owning boundaries. Every
selected peer SHALL verify its own objects and assets; native platform evidence
MAY be supplied by the admitted aggregate executor set without duplicating
unavailable runners or weakening the platform requirement.

macOS release binaries SHALL have an explicitly supplied certificate-bound
identity before archive construction. Signing and archive timestamps SHALL use
the release epoch. Missing signing inputs SHALL stop construction without
prompting, provisioning an identity or substituting ad-hoc signing. Native
acceptance SHALL select only its host operating system; unrelated platform
credentials SHALL NOT be prerequisites. Consuming published assets SHALL NOT
require their private signing keys.

macOS retained-Keychain qualification SHALL exercise partitioned database format
`0x200` and both designated-requirement and application-partition authorization.
A self-signed certificate with a matching designated requirement SHALL NOT count
as proof of stable partition identity across changed code hashes. Release
admission SHALL require an approved Apple-recognized signing identity and an
observed retained-credential transition without prompts or ACL mutation.

#### Scenario: A private Keychain fixture models the system authorization boundary

- **WHEN** native credential tests create an isolated Keychain
- **THEN** they SHALL prove partitioned format and the item's partition ACL
- **AND** a same-byte reader SHALL retain access while a changed self-signed
  code-hash partition SHALL be denied without modifying access policy.

#### Scenario: Certificate-signed archives are rebuilt

- **WHEN** identical source, toolchain, signing inputs and release epoch are built
  on either side of a wall-clock boundary
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** macOS binaries extracted from the archives SHALL satisfy the declared
  native signature requirement.

#### Scenario: Native Linux or Windows acceptance builds its asset

- **WHEN** native acceptance runs without macOS signing inputs
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix.

#### Scenario: Native signing authority is missing

- **WHEN** macOS or complete release construction lacks a required identity,
  password-file or designated-requirement input
- **THEN** it SHALL stop before external build execution
- **AND** existing release output, credentials and host trust SHALL remain intact.

#### Scenario: System Keychain qualification lacks a supplied signing identity

- **WHEN** macOS native acceptance requests system credential-store verification
  without operator-supplied signing inputs
- **THEN** fixture preparation SHALL fail before generating a disposable signing
  identity, constructing an archive or touching the system credential store
- **AND** the diagnostic SHALL identify missing signing authority rather than
  a later credential-read failure
- **AND** supplying inputs SHALL NOT itself establish certificate trust or
  retained-credential authorization.

#### Scenario: both publication planes complete

- **WHEN** GitLab and GitHub independently publish one accepted product release
- **THEN** their commit and annotated tag object identifiers SHALL equal local
  Git exactly
- **AND** their asset manifests and supported-platform semantics SHALL agree.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a selected peer, exact object identity, asset integrity,
  installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT
  claim terminal completion.

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted product object
  and obsolete lanes, policies, compatibility paths, temporary assets, and
  stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each
  independent boundary.

#### Scenario: release metadata exists without publication

- **WHEN** `VERSION` and `CHANGELOG` name a release but either selected peer
  lacks its exact signed tag object, Release record, or assets
- **THEN** delivery SHALL remain incomplete.

## ADDED Requirements

### Requirement: Quality coverage has one positive authority

The architecture policy SHALL assign each tracked carrier exactly one semantic
responsibility. The CI entrypoint and native tool configurations SHALL own the
executable checks and their scopes, collectively covering every applicable
formatting, lint, type, semantic, test, security, and generated-projection
concern. Architecture classification alone SHALL NOT establish quality
coverage. Rules SHALL state the required shape positively; exclusions SHALL
be narrow, justified, and owned by the same authority.

#### Scenario: A tracked carrier is added

- **WHEN** a source, test, configuration, documentation, schema, workflow, or
  release file becomes tracked
- **THEN** architecture classification resolves one responsibility and the
  existing native check scopes cover every applicable concern
- **AND** no file is silently uncovered or governed by competing policies.

#### Scenario: Architecture policy is evaluated

- **WHEN** a policy declares carrier responsibilities and selectors
- **THEN** the architecture gate verifies unique ownership and dependency rules
- **AND** acceptance of check execution requires the separately executed native
  gates, not a duplicate list of check names in that policy.

#### Scenario: A generic checker exists

- **WHEN** a maintained formatter, linter, type, security, dependency, or
  documentation tool covers a required generic concern
- **THEN** the repository uses that tool rather than a custom duplicate
- **AND** retains custom logic only for a documented product invariant.

#### Scenario: A repository npm check executes

- **WHEN** the Go quality gate invokes a formatter, Markdown linter, or OpenSpec
  validator
- **THEN** the locked Node runtime executes its native package script against
  checkout-local dependencies
- **AND** missing dependencies or a nonzero validator exit fail the gate even
  if stdout contains a valid-looking result.

#### Scenario: Native platform source is checked

- **WHEN** native acceptance runs on macOS, Linux, or Windows
- **THEN** the shared Go static-quality policy SHALL execute against that
  host's platform-selected product, tool and test sources before behavioral
  tests and release acceptance
- **AND** a static-check failure SHALL stop those later stages
- **AND** another platform's successful check SHALL NOT substitute for it.

#### Scenario: The complete toolchain is qualified on a native platform

- **WHEN** an operator explicitly selects full native quality
- **THEN** the native entrypoint SHALL execute the existing complete quality
  graph before platform-selected behavioral tests and packaged acceptance
- **AND** each quality command SHALL run once, without a second policy list
- **AND** any failed command SHALL stop all later commands
- **AND** ordinary native acceptance SHALL retain its smaller default path
- **AND** source-signature admission SHALL remain a distinct publication check.

#### Scenario: Quality or release configuration is invalid

- **WHEN** the shared quality command reads golangci-lint or GoReleaser
  configuration
- **THEN** the owning tool's native configuration validator SHALL run before
  source analysis or artifact construction
- **AND** unknown fields, missing input, or a failing validator SHALL fail the
  shared gate even when the file is syntactically valid YAML
- **AND** validation SHALL leave configuration bytes unchanged.

#### Scenario: Markdown configuration is validated before linting

- **WHEN** the shared quality command reads the repository Markdown policy
- **THEN** it SHALL validate options against the CLI schema shipped with the
  locked Markdownlint package and base and override rules against the shipped
  strict built-in-rule schema
- **AND** unknown options or rules, invalid parameter values, multiple YAML
  documents and missing or malformed schema inputs SHALL fail the gate
- **AND** schema references SHALL resolve only from the explicit local inputs,
  without network fetching, copied rule tables or input mutation.

#### Scenario: Current source is scanned for secrets

- **WHEN** the source gate checks credentials in the current checkout
- **THEN** native Gitleaks SHALL scan current regular tracked files, including
  ignored tracked paths, and nonignored untracked files through the existing
  Git inventory
- **AND** path-specific policy and redacted findings SHALL remain effective
- **AND** deleted paths, symlink targets and ignored untracked output SHALL
  remain outside that scope; current-file evidence SHALL NOT imply a history
  scan
- **AND** the scan SHALL preserve source bytes, propagate native failures, and
  reclaim its private input projection after success or failure.

#### Scenario: Markdown sections remain navigable

- **WHEN** current Markdown uses decorative heading punctuation or standalone
  emphasis instead of a section heading
- **THEN** the native Markdown gate SHALL reject the structural defect
- **AND** question headings, emphasized sentences and official OpenSpec
  Goals/Non-Goals labels SHALL remain valid
- **AND** table-column alignment SHALL be checked by the native rule rather
  than a repository-specific parser.

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

### Requirement: Dependency evidence binds the selected lockfiles

The release scanner invocation and report admission SHALL share one exact
lockfile-path selection. Normalization SHALL admit every selected lockfile
exactly once with nonempty package observations before writing either report.

#### Scenario: Scanner output does not establish the selected scope

- **WHEN** an OSV report is empty, partial, duplicated, attributed to another
  checkout or source kind, or lacks package observations
- **THEN** release construction SHALL fail before evidence signing
- **AND** neither normalized report SHALL be written
- **AND** accepted output SHALL remain unchanged and temporary construction
  state SHALL be reclaimed.

### Requirement: Quantitative policy is evidence-derived

Complexity, executable lines, nesting, parameters, coverage, performance, and
test-size thresholds SHALL derive from the risk they protect and the observed
repository distribution. A threshold SHALL include its scope, rationale,
comparison semantics, review condition, and remediation path.

#### Scenario: A threshold changes

- **WHEN** maintainers tighten or relax a quantitative limit
- **THEN** the change records measured evidence and the protected failure mode
- **AND** does not treat a lower number as intrinsically better.

#### Scenario: Multiple rules report one declaration

- **WHEN** independent native rules find violations at the same source line
- **THEN** the Go quality command SHALL retain each rule's diagnostic
- **AND** a combined trial SHALL NOT count a line-deduplicated report as a
  complete per-rule inventory.

#### Scenario: Package evidence cannot be written

- **WHEN** coverage reporting cannot write a measured, zero-statement or
  declaration-only package observation
- **THEN** the gate SHALL fail and preserve the output error
- **AND** it SHALL reclaim its temporary coverage profile without reporting
  successful aggregate acceptance.

### Requirement: Warnings are owned failures

Every repository-owned warning emitted by a supported build, test, analysis,
documentation, packaging, or CI path SHALL be resolved at its semantic owner.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.

#### Scenario: Native validation distinguishes advice from failure

- **WHEN** OpenSpec reports a complete successful validation with only `INFO`
  findings
- **THEN** the gate succeeds and displays every informational finding
- **AND** `WARNING`, `ERROR`, a failed summary, unknown severity or malformed
  evidence still fails admission
- **AND** a diagnostic output failure remains an execution failure.

## REMOVED Requirements

### Requirement: Terminal local release readiness

**Reason:** Release admission is duplicated across capabilities and this copy permits peer-local re-signing and an unavailable branch metric.

**Migration:** Use product-control-plane / Terminal local release readiness for source admission, faithful quantitative quality evidence for coverage, and Independent Forge parity for unchanged signed objects. The npm bootstrap and publication scenarios are retained in the source-admission owner.

### Requirement: accepted ref parity is visible without duplicate proof

**Reason:** This requirement suppresses accepted dev events and conflicts with the admitted lifecycle-scoped CI graph.

**Migration:** Use ci-diagnostics / Every integration path produces exact-commit evidence. Review and accepted-branch events each run; release pushes additionally verify ref parity. No cross-event evidence-reuse capability is implied.

## RENAMED Requirements

- FROM: `### Requirement: Accepted publication trees contain only archived Changes`
- TO: `### Requirement: Source acceptance precedes delivery completion`
