# product-quality Specification

## Purpose
TBD - created by archiving change terminal-quality-convergence. Update Purpose after archive.
## Requirements
### Requirement: one complete quality graph

The repository SHALL expose one logical quality graph reused by local
development, exact-HEAD governance proof, GitLab CI, and GitHub Actions. One
declarative topology authority SHALL generate every Forge-native CI file.
Generated files SHALL NOT own policy. Executable behavior SHALL remain in
repository-owned commands. Projection drift SHALL fail before other source
gates. Product targets, release assets, native acceptance, and developer-host
compatibility SHALL be distinct claims. Cross-compilation SHALL prove only
asset production and SHALL NOT substitute for native execution evidence.

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

- **WHEN** a required macOS, Linux, or Windows native job cannot execute
- **THEN** CI SHALL report the missing evidence within a bounded interval
- **AND** no other Forge result or cross-compile SHALL substitute for the native check.

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

### Requirement: portable repository text

The repository SHALL define editor intent, checkout normalization, quality
execution, build, installation, and release behavior using tracked,
platform-neutral configuration. The contract SHALL run through the same
repository-native command on macOS, Linux, and Windows without requiring a
particular interactive shell, personal path, workstation product, IDE, or
foreign repository.

Repository-owned paths SHALL use slash-separated repository semantics until the
filesystem boundary. A host adapter SHALL convert them exactly once after
rejecting traversal, absolute, backslash, and volume-qualified input. Tool
processes SHALL use repository-relative inputs from an explicit repository
working directory when absolute paths can cross host volumes.

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic

#### Scenario: Native Windows renders the CI projection

- **WHEN** the repository and a test fixture reside on different Windows volumes
- **THEN** CUE evaluates `.config/ci/pipeline.cue` from the selected repository root
- **AND** generated `.github` and GitLab paths resolve within that root
- **AND** the same focused contracts pass on macOS, Linux, and Windows

### Requirement: actor-independent contribution policy

The repository SHALL require structured commit messages and trusted signatures
without binding a personal name, email, key, fingerprint, host path, signing
program, or Forge credential. Each Forge's protected context SHALL supply its
own actor and trust input independently.

#### Scenario: an admitted team contributor commits

- **WHEN** a commit enters protected history
- **THEN** its message and signature SHALL satisfy repository policy using the explicit publication context for that Forge

#### Scenario: one Forge is unavailable

- **WHEN** GitLab or GitHub cannot verify or publish
- **THEN** the other Forge's verification and publication SHALL remain independently executable and SHALL NOT claim success for the unavailable plane

### Requirement: faithful quantitative quality evidence

The repository SHALL measure statement coverage and branch coverage
independently. Each measure SHALL be strictly greater than 95 percent for every
package and for the module aggregate. Package completeness SHALL prove that
every package selected by the canonical module query appears exactly once in
both results. Evidence SHALL retain package identity, raw covered and total
counts, source revision and tree, analyzer identity, and policy digest; one
measure SHALL NOT be inferred from another.

#### Scenario: any quantitative boundary is not met

- **WHEN** a package or aggregate has statement or branch coverage of 95 percent or less, is absent, is duplicated, or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before promotion

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid

### Requirement: bounded semantic structure

Production, test, and repository-tool code SHALL follow the declared semantic
topology and dependency direction. Enforced limits SHALL cover file and
directory size, function size, decision complexity, nesting, naming, package
documentation, and import ownership. Composition roots SHALL assemble cohesive
owners; shared behavior SHALL live at the smallest stable semantic owner rather
than in a forwarding wrapper, alias-only package, or copied helper.

#### Scenario: structure exceeds an admitted bound

- **WHEN** production, test, or tool code exceeds a declared structural limit or violates semantic ownership
- **THEN** the architecture gate SHALL fail with the exact owner and measured bound

#### Scenario: an ordinary provider is added

- **WHEN** a provider can be represented by Account, endpoint, Profile, and Route data
- **THEN** it SHALL require no provider-specific command, client projection, installer, service lifecycle, or core dependency branch

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent Forge publication, asset
integrity, installation, runtime acceptance, and repository housekeeping. A
release SHALL be complete only when its signed tag, immutable assets, checksums,
Forge-native release record, and supported-platform acceptance are verified
independently on each declared publication plane.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a publication plane, asset parity, installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT claim terminal completion.

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted head and obsolete lanes, policies, compatibility paths, temporary assets, and stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each independent boundary.

#### Scenario: release metadata exists without publication

- **WHEN** VERSION and CHANGELOG name a release but either Forge lacks its signed tag, release record, or assets
- **THEN** delivery SHALL remain incomplete.

#### Scenario: both publication planes complete

- **WHEN** GitLab and GitHub independently verify and publish the same source tree
- **THEN** their commit and tag object identifiers MAY differ
- **AND** their source tree, version, asset manifest, and supported-platform semantics SHALL agree.

### Requirement: Independent Forge parity

GitLab and GitHub SHALL remain independent publication planes. A provider-specific identity projection SHALL prove exact accepted tip-tree parity and independently verify the complete target commit provenance and release-tag trust. Deterministic collapse of semantically duplicate source commits SHALL NOT be treated as source drift.

#### Scenario: Equivalent provider projection

- **WHEN** provider identity normalization maps duplicate semantic commits or parents to the same target object
- **THEN** the projected branch is accepted only when its tip tree exactly equals the canonical accepted tip tree and all provider-native provenance checks pass

#### Scenario: Real source drift

- **WHEN** a projected branch tip resolves to a different tree
- **THEN** synchronization fails before publication

### Requirement: Portable exact-version CI bootstrap

The GitLab Linux bootstrap SHALL derive its mise version from the repository lock authority and SHALL use bounded, retryable HTTP transport without querying a foreign Forge API.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the bootstrap retries a bounded number of times over HTTP/1.1 and still verifies the upstream release checksum before installation
