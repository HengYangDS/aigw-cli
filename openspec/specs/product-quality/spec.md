# product-quality Specification

## Purpose

Define the product invariants that make AIGW verifiable, portable, and
independently publishable without turning presentation preferences or
repository-specific measurements into arbitrary merge vetoes.
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

Tracked text SHALL use deterministic encoding and line-ending semantics across
supported hosts. Repository-owned verification SHALL express host-independent
behavior with one semantic contract on macOS, Linux, and Windows. A test SHALL
not infer a portable I/O failure from POSIX permission bits when the target host
does not implement that permission model.

#### Scenario: a native test proves a filesystem error boundary

- **WHEN** repository verification must exercise a filesystem operation failure
- **THEN** the fixture SHALL construct that failure deterministically on every supported host
- **AND** the same production error path SHALL be asserted without a platform skip

#### Scenario: text contains a byte-level defect

- **WHEN** tracked text contains CR line endings, trailing whitespace, or lacks a final newline
- **THEN** repository quality SHALL report the exact file and line

#### Scenario: a contributor uses another supported host

- **WHEN** the repository is cloned under another operating system, user, directory, or Forge
- **THEN** checkout, verification, build, installation, and release contracts SHALL remain discoverable and executable from repository-owned inputs

#### Scenario: a contributor uses another operating system

- **WHEN** Git checks out tracked text
- **THEN** line endings and executable semantics SHALL remain deterministic

#### Scenario: Native Windows renders the CI projection

- **WHEN** the repository and a test fixture reside on different Windows volumes
- **THEN** CUE SHALL evaluate the CI authority from the selected repository root
- **AND** generated Forge paths SHALL resolve within that root
- **AND** the same focused contracts SHALL pass on macOS, Linux, and Windows

#### Scenario: valid text uses a different readable spacing style

- **WHEN** Markdown or configuration uses semantically valid blank-line spacing
- **THEN** the repository-wide byte checker SHALL not reject it
- **AND** formatters, serializers, and review SHALL retain their own scoped authority

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

The repository SHALL measure statement and branch coverage independently. The
canonical machine policy SHALL own the aggregate floor, package-observation contract, comparison
semantics, risk model, false-positive cost, remediation path, and review
condition. Every production package SHALL appear in the evidence, execute its
owned statements and branches, and retain exact diagnostic ratios. Branchless
packages SHALL remain visible and report 100-percent branch coverage. Evidence
SHALL retain raw counts, package identity, source revision and tree, analyzer
identity, and policy digest. No prose, CI projection, or tool-native formatting
file SHALL own a competing threshold.

#### Scenario: quantitative evidence is evaluated

- **WHEN** coverage is admitted for promotion
- **THEN** aggregate statement and branch evidence SHALL each be strictly greater than 95 percent
- **AND** every canonical production package SHALL be present in the same complete evidence set
- **AND** every package SHALL remain executed and report exact statement and branch ratios
- **AND** the verdict SHALL be independent of duplicated literals or inferred metrics.

#### Scenario: a quantitative boundary or observation contract is not met

- **WHEN** an aggregate ratio is equal to or below the canonical floor, or a package is absent, wholly unexecuted, duplicated, or lacks bound raw evidence
- **THEN** local verification, exact-HEAD proof, and hosted CI SHALL fail before promotion.

#### Scenario: statement data is presented as branch evidence

- **WHEN** a result derives a branch claim from a statement-only profile
- **THEN** the evidence SHALL be rejected as semantically invalid.

#### Scenario: a package owns no branches

- **WHEN** the branch analyzer reports a present canonical package with zero branch decisions
- **THEN** that package SHALL remain visible with a 100-percent branch ratio
- **AND** it SHALL NOT be treated as absent or unexecuted.

#### Scenario: aggregate coverage carries the quantitative veto

- **WHEN** a package has a small or volatile denominator while the aggregate floor passes
- **THEN** the exact package ratio SHALL remain visible for review
- **AND** the package ratio SHALL NOT independently veto an otherwise valid aggregate result.

### Requirement: semantic structure

Production, test, and repository-tool code SHALL follow the declared semantic
topology and dependency direction. Naming, import ownership, and composition
roots SHALL express cohesive semantic owners. Shared
behavior SHALL live at the smallest stable owner rather than in a forwarding
wrapper, alias-only package, or copied helper. Size, complexity, nesting, and
other presentation heuristics MAY inform review, but SHALL NOT reject a change
without an independently justified risk model, defined measurement semantics,
false-positive cost, remediation path, and review trigger.

The machine architecture policy SHALL state that rationale once for its positive
contract. Its checker SHALL derive repository identity from canonical project
metadata and SHALL NOT encode product names, authors, hosts, languages, shells,
addresses, runner inventories, or historical implementation shapes as generic
merge blacklists.

#### Scenario: structure violates semantic ownership

- **WHEN** production, test, or tool code violates the declared topology,
  dependency direction, composition root, naming, or ownership contract
- **THEN** the architecture gate SHALL fail with the exact semantic violation.

#### Scenario: a heuristic changes

- **WHEN** a size, complexity, nesting, or presentation heuristic is proposed
  as a merge condition
- **THEN** it SHALL remain review evidence unless its risk model, measurement,
  false-positive cost, remediation path, and review trigger are admitted.

#### Scenario: an ordinary provider is added

- **WHEN** an ordinary provider implementation is added below the declared
  provider owner without changing package topology or dependency direction
- **THEN** no repository-shape allowance or threshold change SHALL be required.

#### Scenario: an implementation technique changes

- **WHEN** a bounded change introduces another language, shell carrier, alias,
  adapter, address example, or local path example
- **THEN** that syntax alone SHALL NOT decide merge admission
- **AND** the positive owner, dependency, portability, security, and evidence
  contracts SHALL determine the verdict.

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

### Requirement: Forge capability projection

The repository SHALL expose one product evidence graph and one deterministic CI
topology authority. Product evidence requirements SHALL be modeled separately
from each Forge's admitted executor capacity. A Forge projection SHALL contain
only native jobs that Forge can execute, while the aggregate product evidence
set SHALL retain every supported native platform. A missing executor on one
Forge SHALL NOT create an optional, indefinitely pending, or `allow_failure`
substitute and SHALL NOT weaken product support.

#### Scenario: one Forge lacks a Windows executor

- **WHEN** another independent publication plane supplies admitted native Windows evidence
- **THEN** the Forge without Windows capacity SHALL omit its Windows job
- **AND** the product evidence model SHALL continue to require Windows
- **AND** cross-compilation SHALL NOT be reported as native evidence.

#### Scenario: Windows capacity is admitted later

- **WHEN** GitLab gains a qualified Windows executor
- **THEN** one capability declaration SHALL restore the generated native Windows job
- **AND** no parallel workflow or compatibility switch SHALL be introduced.

### Requirement: Terminal local release readiness

AIGW SHALL advance a release candidate to current stable releases across the
application, test, and declared Go tool closure before freezing that candidate.
`go.mod` and `go.sum` SHALL remain the sole dependency selection authority;
modules outside the compiled package and tool closure are not selected supply
chain inputs. Aggregate statement and branch coverage SHALL remain strictly
greater than 95 percent; every package SHALL remain present, executed, and
reported. The native source and release gates SHALL pass before publication.

#### Scenario: Stable dependency updates are available

- **WHEN** the compiled application, test, or declared tool closure reports newer stable releases
- **THEN** AIGW SHALL refresh `go.mod` and `go.sum` together
- **AND** SHALL run the complete source, coverage, and release gates
- **AND** SHALL NOT preserve the older graph as a compatibility target

#### Scenario: The verified release is published

- **WHEN** exact-HEAD proof passes for the refreshed source tree
- **THEN** GitLab and GitHub MAY construct their own signed commit and tag provenance
- **AND** both Forge histories SHALL represent the same verified source tree
- **AND** each Forge SHALL publish its complete release asset matrix independently
