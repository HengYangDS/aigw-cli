# product-quality Specification

## Purpose

Define the product invariants that make AIGW verifiable, portable, and
independently publishable without turning presentation preferences or
repository-specific measurements into arbitrary merge vetoes.

## Requirements

### Requirement: one complete quality graph

The repository SHALL expose one quality graph reused by local development,
exact-HEAD proof, GitLab, and GitHub. One declarative topology SHALL generate
Forge files; generated files MUST NOT own policy, and repository commands SHALL
own behavior. Projection drift fails first. Product targets, release assets,
native acceptance, and host compatibility are distinct claims;
cross-compilation proves only artifacts. A Forge MUST run the graph at most
once per commit and lifecycle stage.

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

#### Scenario: a developer submits a proposal for review

- **WHEN** a proposal targets `dev` through a pull request or merge request
- **THEN** the review head SHA SHALL receive the complete verification graph
- **AND** the proposal branch push SHALL NOT start a parallel copy of that graph.

#### Scenario: a maintainer publishes an accepted product object

- **WHEN** local accepted `main` is projected unchanged to peer `main` and `dev`
- **THEN** the peer SHALL verify the `main` publication
- **AND** the equal `dev` projection SHALL NOT start another copy of the graph.

#### Scenario: explicit diagnosis is required

- **WHEN** a maintainer explicitly dispatches verification
- **THEN** the selected Forge SHALL run the complete graph for the selected ref.

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
program, or Forge credential in source. Product identity and trust SHALL be
explicit publication inputs; each peer SHALL independently supply only its
transport credential and hosted account verification.

#### Scenario: an admitted team contributor commits

- **WHEN** a commit enters protected product history
- **THEN** its unchanged message, identities, and signature SHALL satisfy the
  explicit product trust policy.

#### Scenario: one Forge is unavailable

- **WHEN** GitLab or GitHub cannot verify or publish
- **THEN** local verification and the other peer SHALL remain independently
  executable and SHALL NOT claim success for the unavailable peer.

### Requirement: faithful quantitative quality evidence

Statement and branch coverage SHALL be measured independently under one machine
policy owning the aggregate floor, package observation, comparison, risk,
remediation, and review. Every production package MUST appear, execute its
owned statements and branches, and retain exact ratios; branchless packages
report 100-percent branch coverage. Evidence MUST bind raw counts, package,
revision and tree, analyzer, and policy digest. No prose, CI projection, or
formatter may own another threshold.

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

All code SHALL follow declared semantic topology, dependency direction, naming,
import ownership, and composition roots. Shared behavior belongs to the
smallest stable owner, never a forwarding wrapper, alias-only package, or
copied helper. Size and complexity MAY guide review but MUST NOT gate without a
justified risk model, measurement, false-positive cost, remediation, and
trigger. One machine policy SHALL state this positive contract without
repository-specific merge blacklists.

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
graph, exact-HEAD proof, native hosted CI, independent peer publication, exact
branch and tag identity, asset integrity, installation, runtime acceptance, and
repository housekeeping. A release SHALL be complete only when its one signed
tag object, immutable assets, checksums, peer-native Release records, and
supported-platform acceptance are verified independently on each declared
publication plane.

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

The GitLab Linux bootstrap SHALL derive its mise version from the repository lock authority and SHALL use bounded, retryable HTTP transport without querying a foreign Forge API.

#### Scenario: Transient HTTP transport failure

- **WHEN** an installer or asset transfer encounters a transient transport error
- **THEN** the bootstrap retries a bounded number of times over HTTP/1.1 and still verifies the upstream release checksum before installation

### Requirement: Forge capability projection

One product evidence graph and deterministic CI topology SHALL separate product
evidence from each Forge's executor capacity. A Forge projection MUST contain
only native jobs it can run, while aggregate evidence retains every supported
platform. Missing capacity on one Forge MUST NOT create optional, indefinitely
pending, or `allow_failure` substitutes, weaken product support, or let
cross-compilation stand in for native evidence.

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

AIGW SHALL update application, tests, and repository tools to stable releases
before freezing a candidate. `go.mod` with `go.sum`, `mise.toml` with
`mise.lock`, and `package.json` with `package-lock.json` own the Go, tool, and
npm closures; a clean runner MUST use the npm lock. Bound aggregate statement
and branch coverage MUST exceed 95 percent, every package remain present and
executed, and native source and release gates pass before publication.

#### Scenario: Stable dependency updates are available

- **WHEN** the application, test, or declared repository-tool closure reports newer stable releases
- **THEN** AIGW SHALL refresh the owning ecosystem declaration and lock together
- **AND** SHALL run the complete source, coverage, and release gates
- **AND** SHALL NOT preserve the older graph as a compatibility target

#### Scenario: A clean runner materializes npm tools

- **WHEN** source verification starts without an existing npm installation
- **THEN** the runner SHALL install the exact committed npm dependency graph
- **AND** install scripts SHALL remain disabled
- **AND** no direct or transitive npm version SHALL be selected outside `package-lock.json`
- **AND** registry signatures SHALL verify through the ecosystem's standard verifier

#### Scenario: The verified release is published

- **WHEN** exact-HEAD proof passes for the refreshed source tree
- **THEN** GitLab and GitHub MAY construct their own signed commit and tag provenance
- **AND** both Forge histories SHALL represent the same verified source tree
- **AND** each Forge SHALL publish its complete release asset matrix independently

### Requirement: Accepted publication trees contain only archived Changes

The source gate SHALL admit an accepted publication tree only when
`openspec/changes/` contains no active Change directories. Completed Change
artifacts SHALL be archived before `dev`, `main`, or a release tag is accepted.

#### Scenario: Active Change reaches source verification

- **WHEN** source verification observes an active Change directory
- **THEN** verification SHALL fail with the active Change names
- **AND** SHALL direct the maintainer to archive completed Changes before publication

### Requirement: accepted ref parity is visible without duplicate proof

When a maintainer publication atomically advances peer `main` and `dev` to one
accepted product object, each peer SHALL expose one `main` result that proves
both protected refs resolve to the event's exact commit. The same `main` event
SHALL own the complete verification graph. A proposal merge into `dev` SHALL
remain a reviewed development state and SHALL NOT be interpreted as accepted
publication merely because it updates the protected review branch.

#### Scenario: a developer proposal is merged into dev

- **WHEN** a reviewed proposal advances peer `dev`
- **THEN** the completed pull-request or merge-request graph SHALL remain its
  verification evidence
- **AND** the resulting `dev` push SHALL NOT require `main` parity
- **AND** the resulting `dev` push SHALL NOT repeat the complete graph.

#### Scenario: a peer receives one accepted object on main and dev

- **WHEN** an atomic maintainer publication advances peer `main` and `dev`
- **THEN** the `main` event SHALL run the complete verification graph
- **AND** the same `main` event SHALL require both protected refs to equal its
  exact commit
- **AND** no second graph or parity-only `dev` pipeline SHALL be required.

### Requirement: repository text quality has one mature owner per concern

Portable byte invariants SHALL be declared in `.editorconfig` and verified by a
locked cross-platform EditorConfig implementation. Current product Markdown
SHALL be formatted by Prettier, linted by markdownlint, and checked for explicit
link validity by lychee. Repository-specific analyzers SHALL NOT duplicate
those responsibilities or infer links that an author did not declare.
Immutable OpenSpec archives SHALL remain outside current-document rewriting.

#### Scenario: a current text artifact violates its declared contract

- **WHEN** a tracked text file violates `.editorconfig` or a current Markdown
  file violates the locked formatter, linter, or explicit-link contract
- **THEN** repository verification SHALL reject the exact artifact
- **AND** the responsible mature tool SHALL emit the diagnostic.
