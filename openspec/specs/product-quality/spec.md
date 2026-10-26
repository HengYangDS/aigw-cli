# product-quality Specification

## Purpose

Define the product invariants that make AIGW verifiable, portable, and
independently publishable without turning presentation preferences or
repository-specific measurements into arbitrary merge vetoes.

## Requirements

### Requirement: one complete quality graph

One quality graph SHALL serve local development, exact-HEAD proof, GitLab, and
GitHub. A declarative topology generates Forge files; repository commands own
behavior and projection drift SHALL fail first. Product targets, artifacts,
native acceptance, and host compatibility SHALL remain distinct claims. Each
admitted event SHALL receive its required graph; cross-event reuse requires an
explicit verifier, and cross-compilation SHALL prove artifacts only.

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

- **WHEN** a selected Forge lacks an admitted executor for a supported platform
- **THEN** its required job SHALL remain in the projection and that peer SHALL
  not be accepted as green
- **AND** another peer's result, operating system, or cross-compile SHALL NOT
  substitute for that peer-local native job.

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

- **WHEN** a maintainer explicitly dispatches verification without a platform selector
- **THEN** the selected Forge SHALL run the complete graph for the selected ref
- **AND** a targeted platform dispatch SHALL remain diagnostic rather than
  complete review or release evidence.

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

One machine policy SHALL own Go statement coverage, including the aggregate
floor, package observation, comparison, risk, remediation, and review. Every
production package MUST remain visible; measurable packages MUST execute owned
statements with exact ratios, while proven zero-statement packages MUST report
not applicable. Evidence MUST bind raw counts, package, revision, tree,
toolchain, and policy. Statement data SHALL NOT be relabeled as branch coverage.

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

### Requirement: Source acceptance precedes delivery completion

Source acceptance SHALL require valid Change artifacts, exact-source quality,
and authorized object-preserving integration. Active Changes MAY reach accepted
or release refs before delivery completes. Tasks SHALL close implementation and
candidate acceptance before archive; canonical specs and design SHALL retain
delivery duties until proved. Archive SHALL precede stable tagging without
claiming delivery or blocking local integration. OpenSpec SHALL remain the sole
Change-artifact validator.

#### Scenario: Active Change reaches source verification

- **WHEN** source verification observes an active Change in a work lane,
  proposal, accepted branch or release branch
- **THEN** the same official artifact validation and product quality graph run
- **AND** the presence of its task carrier alone SHALL NOT reject valid source
- **AND** malformed artifacts or failed quality checks still block acceptance
- **AND** source acceptance SHALL NOT mark pending publication, installation or
  cleanup complete.

#### Scenario: Archive preserves pending delivery

- **WHEN** source and candidate tasks are complete but a peer's CI, assets,
  installed lifecycle or retirement remains pending
- **THEN** archive SHALL retain those duties in canonical specs and design
- **AND** it SHALL NOT claim the pending external delivery is complete.

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

### Requirement: Quality coverage has one positive authority

Architecture policy SHALL assign each tracked carrier one semantic owner.
The CI entrypoint and native tool configurations SHALL own executable scopes
covering every applicable format, lint, type, semantic, test, security, and
projection concern. Classification alone SHALL NOT prove execution. Rules SHALL
state positive required shape; exclusions SHALL remain narrow, justified, and
owned by the same authority.

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
exactly once with nonempty package observations and established license
identities before writing either report; an exact version-bound OSV license
override MAY correct a verified upstream metadata gap without suppressing
vulnerability observations. Scanning and admission SHALL precede artifact
construction.

#### Scenario: Dependency admission fails before construction

- **WHEN** the scanner fails, its output is malformed, or its scope is incomplete
- **THEN** GoReleaser, SBOM generation and signing SHALL NOT start
- **AND** accepted output and cleanup obligations SHALL remain unchanged.

#### Scenario: Scanner output does not establish the selected scope

- **WHEN** an OSV report is empty, partial, duplicated, attributed to another
  checkout or source kind, or lacks package observations
- **THEN** release construction SHALL fail before evidence signing
- **AND** neither normalized report SHALL be written
- **AND** accepted output SHALL remain unchanged and temporary construction
  state SHALL be reclaimed.

#### Scenario: Scanner reports an unestablished license

- **WHEN** an observed package has no license or reports a blank, `UNKNOWN`,
  `NOASSERTION`, or `NONE` license identity
- **THEN** release construction SHALL reject the dependency report before
  writing normalized evidence
- **AND** neither normalized license nor vulnerability evidence SHALL be signed.

#### Scenario: Locked package has incomplete upstream license metadata

- **WHEN** a maintainer verifies the license in an integrity-locked package but
  OSV reports an unestablished license
- **THEN** the selected OSV policy MAY override only that exact ecosystem,
  package name, and version
- **AND** the override SHALL leave vulnerability scanning active
- **AND** a later package version SHALL require a new license observation.

#### Scenario: The Go standard library is part of the scan

- **WHEN** release construction scans `go.mod`
- **THEN** the selected OSV policy SHALL keep the Go standard library at the
  module's Go version in the scanned inventory
- **AND** construction SHALL fail before signing unless each release program in
  the SBOM carries exactly one package of the main module that `go.mod`
  declares and exactly one standard library, at that version, as a dependency
  of that same program's main-module package.

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

Repository-owned source and configuration warnings SHALL block qualification
until resolved at their semantic owner. Supply-chain security findings SHALL
remain nonblocking and fully disclosed under the
[release evidence contract](../../../docs/governance/change-and-release-policy.md#reproducible-assets).
Scanner execution, evidence completeness and artifact validity SHALL remain
required. Successful subprocess exit SHALL NOT erase captured diagnostics or
establish warning-free acceptance.

#### Scenario: A supported gate emits a warning

- **WHEN** the warning is attributable to repository source or configuration
- **THEN** the gate fails until the cause is removed
- **AND** a blanket filter, baseline, or ignored exit code is not accepted as
  the repair.

#### Scenario: Native validation distinguishes advice from failure

- **WHEN** OpenSpec reports `INFO`, `WARNING`, `ERROR`, or a failed summary
- **THEN** the repository gate fails and reports every finding without filtering
  it, even if the native validator classifies the item as informational
- **AND** unknown severity, malformed evidence, or diagnostic output failure
  also fails admission.

#### Scenario: Successful child diagnostics remain observable

- **WHEN** a supported subprocess writes stdout and stderr before exiting zero
- **THEN** two-stream capture SHALL preserve both bounded byte strings separately
- **AND** native journey evidence SHALL expose redacted diagnostic output
- **AND** a stdout-only API MAY retain its explicit result-only success contract.

#### Scenario: Successful quality tool emits a native diagnostic

- **WHEN** a supported CI tool exits zero with explicit native stderr warning or error markers
- **THEN** the common command runner SHALL reject qualification using the existing diagnostic classifier
- **AND** truncated diagnostic evidence SHALL prevent a warning-free success claim
- **AND** ordinary progress and zero-finding count fields SHALL remain admissible
- **AND** complete diagnostics SHALL remain observable without an additional parser or warning authority.

#### Scenario: A native tool exits before diagnostic pipe writes drain

- **WHEN** a native CI tool immediately exits after writing stderr
- **THEN** its complete diagnostic stream SHALL remain observable after exit
- **AND** standard output SHALL retain its live execution contract
- **AND** capture, read, replay or cleanup failure SHALL prevent qualification
- **AND** exact temporary capture files SHALL be reclaimed without changing unrelated files.

#### Scenario: Successful native inference emits a capability warning

- **WHEN** Codex, Claude or Hermes exits zero but reports an explicit native stderr warning, error or traceback
- **THEN** public verification SHALL report incomplete qualification without exposing private diagnostics
- **AND** verification SHALL NOT write a successful checkpoint or claim unproved inference
- **AND** missing native model metadata SHALL remain visible without fabricated catalogue entries
- **AND** ordinary version, session and progress stderr SHALL NOT become a warning failure.

#### Scenario: Native Codex rejects an inherited temporary-home relationship

- **WHEN** catalogue or verification runs under an inherited temporary root
- **THEN** its native child SHALL receive an owned temporary root that does not contain its selected Codex home
- **AND** parent environment and user configuration SHALL remain unchanged
- **AND** native catalogue warnings SHALL prevent qualification rather than silently discard diagnostics
- **AND** failed and successful probes SHALL reclaim their exact owned workspace.

#### Scenario: Native rule conformance exceeds its aggregate execution budget

- **WHEN** independent positive and negative Go rule fixtures qualify the same policy
- **THEN** one bounded native batch SHALL execute every isolated fixture package
- **AND** valid fixtures SHALL have no findings and each invalid fixture SHALL retain its intended diagnostic
- **AND** root resolution, format and type-failure contracts SHALL remain independently exercised
- **AND** policy thresholds, native diagnostics and the original package timeout SHALL NOT be weakened.

### Requirement: Terminal layout follows semantic fields and display width

Human output SHALL use one presentation owner for terminal-cell measurement,
ANSI-aware layout, wrapping, and aligned semantic fields. Groups stack when
content cannot fit; metadata SHALL own executable names and pflag SHALL own
option grammar. Color SHALL NOT alter alignment. Machine, helper, and interactive
contracts SHALL remain unchanged. Copyable commands SHALL preserve characters,
whitespace, and logical lines; only explanatory text MAY wrap, without adding
shell syntax.

#### Scenario: Copy an executable command from a narrow terminal

- **WHEN** an executable example or follow-up command exceeds the terminal width
- **THEN** its output SHALL preserve the command bytes apart from presentation
  indentation, optional styling and the final output newline
- **AND** neither quoted whitespace nor argument tokens SHALL be rewritten to
  satisfy an explanatory-text width rule.

#### Scenario: Starting commands have different lengths

- **WHEN** root help presents the setup, selection and readiness journey
- **THEN** command and explanation SHALL be distinct fields with one description
  column derived from the group's longest label
- **AND** a renamed root command SHALL propagate without hard-coded product text.

#### Scenario: A terminal is narrow or a value spans multiple lines

- **WHEN** human output contains long labels, unbroken values or multiple lines
- **THEN** the renderer SHALL retain text and continuation indentation within the
  available display width and use stacked rows where necessary
- **AND** colored and plain output SHALL preserve the same layout.

#### Scenario: Inspect the complete public command tree

- **WHEN** every public command renders help at supported narrow and wide widths
- **THEN** headings, descriptions, usage grammar and options SHALL fit those widths
- **AND** native option meaning and credential-free help SHALL remain intact.

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
runtime acceptance, and housekeeping.

Release completion SHALL bind one signed tag to immutable assets, checksums,
native Release records, and supported-platform results. Each peer verifies its
own objects and assets. The aggregate executor set MAY supply native platform
evidence without duplicating unavailable runners.

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

### Requirement: Native Go events retain one coverage attempt

The existing coverage command SHALL optionally emit original Go test JSON on
stdout and coverage summaries on stderr. Tests and retained coverage counters
SHALL originate from one native invocation with the selected package inventory,
tags, race option, atomic instrumentation and threshold policy unchanged.

#### Scenario: Machine-readable coverage succeeds

- **WHEN** coverage selects `--json` and an explicit profile destination
- **THEN** stdout SHALL contain only the native test event bytes
- **AND** package and aggregate summaries SHALL remain separately observable
- **AND** the retained profile SHALL contain the same attempt's counters.

#### Scenario: The native test attempt fails

- **WHEN** the selected native tests fail after emitting events and a profile
- **THEN** their event bytes and retained diagnostic counters SHALL remain observable
- **AND** the command SHALL fail without qualifying partial coverage
- **AND** temporary profile cleanup SHALL remain required.

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

### Requirement: Native Markdown formatting preserves semantic containers

Current authored Markdown SHALL use the locked original Prettier formatter with
preserved prose wrapping and native embedded-language formatting. The existing
fix and check commands SHALL consume the same declared inventory without ambient
configuration. Other lint rules SHALL accept formatter-canonical containers;
immutable archives and native byte-exact ignores SHALL remain unchanged.

#### Scenario: Canonical lists retain their semantic separation

- **WHEN** native Prettier formats tight, loose, nested or quoted lists, including
  wrapped paragraphs, fenced examples and tables
- **THEN** the existing check SHALL accept the native output without a spacing
  postprocessor or custom paragraph parser
- **AND** native ignored examples and unrelated source SHALL retain their bytes.

#### Scenario: A real formatting journey converges once

- **WHEN** authored inputs differ from their native canonical output
- **THEN** the check SHALL refuse without mutation
- **AND** the fix SHALL produce the native output on that same inventory
- **AND** a second fix SHALL leave every byte unchanged and the check SHALL pass.

### Requirement: Human output groups share one value column

A command SHALL submit each group of fields, with their state symbols and
details, as one unit. The group SHALL share one value column at least two cells
after its longest label and symbol; values and details continue under it, and
the group stacks as a whole only when too little width remains for values.
Standalone notes SHALL keep one indentation at every width, and explanatory
text SHALL break only at spaces unless one word exceeds the width.

#### Scenario: Classified rows have labels of different lengths

- **WHEN** a section such as the doctor checks lists rows whose labels differ in
  length or exceed the default label column
- **THEN** every value of the group SHALL start in one column at least two
  cells after the longest label and its state symbol
- **AND** a value or detail wider than the remaining width SHALL continue under
  that column instead of stacking its row alone.

#### Scenario: Human output has no known width

- **WHEN** human output is piped or the terminal width is unknown
- **THEN** groups and notes SHALL keep the same columns and indentation as at a
  known width, without wrapping.

#### Scenario: Wrapped text contains hyphenated identifiers

- **WHEN** explanatory text containing client, Route or model identifiers wraps
- **THEN** lines SHALL break only at spaces, keeping each identifier whole unless
  it alone exceeds the available width.

#### Scenario: Help lists command options

- **WHEN** command help lists its options
- **THEN** option lines SHALL start at the indentation of the other section
  entries and keep pflag's own layout.

### Requirement: Human output names clients and protocols by their labels

Human output SHALL name each client by its registry display label and each
protocol by its protocol label, in rows, prose and problems alike, and SHALL
NOT capitalize an identifier into a third name. Help SHALL list the
identifiers that a flag accepts, and copyable commands SHALL keep identifiers.

#### Scenario: A client with a multi-word label appears in human output

- **WHEN** check, test, verify, status, sync, repair, migration, selection,
  route or account output names Claude Desktop
- **THEN** it SHALL use `Claude Desktop`
- **AND** JSON output SHALL keep the identifier `claude-desktop`.

#### Scenario: Help lists client choices

- **WHEN** help describes a `--for` flag
- **THEN** it SHALL list the identifiers that the flag admits.

#### Scenario: Human output shows a protocol

- **WHEN** selection or migration output shows a binding's protocol
- **THEN** it SHALL use the protocol label, such as `OpenAI Responses`.

### Requirement: Native warning symbols are diagnostics

A native diagnostic line whose first visible character is an explicit warning
or error symbol, `⚠`, `✗`, `✖`, `✘` or `❌`, SHALL count as a warning in
every check that rejects explicit native warnings. Check marks, progress
glyphs and symbols inside a line SHALL NOT.

#### Scenario: Hermes reports an out-of-sync installation

- **WHEN** Hermes exits zero after printing a line that starts with `⚠`
- **THEN** verification SHALL report incomplete qualification without the
  private diagnostic.

#### Scenario: A client prints ordinary progress

- **WHEN** a client prints `session_id:`, a `✓` result or a `↻` resume line
- **THEN** that line SHALL NOT be a warning.

### Requirement: The Hermes release journey resumes its session natively

The native release journey SHALL create its Hermes session once with its
creation options, resume it by the stored session identifier without options
that Hermes freezes at creation, and stop the journey's gateway before
deleting the journey home.

#### Scenario: Continue the Hermes session across program lifecycle steps

- **WHEN** the journey continues its Hermes session after an update, rollback
  or re-upgrade
- **THEN** it SHALL pass `--resume` with the stored session identifier
- **AND** Hermes SHALL retain the earlier tool result in the continued input.

### Requirement: Diagnostics name the native file they inspect

When doctor reports an observation about one native client configuration file
under the user's home directory, human and JSON output SHALL name that file by
its path relative to the home directory. A file outside the home directory
SHALL NOT be named, and its human label SHALL say that it lies outside the home
directory. Diagnostics SHALL NOT expose an absolute or other private path.

#### Scenario: Codex projects into its default home

- **WHEN** doctor inspects a Codex binding whose target is
  `~/.codex/config.toml`
- **THEN** the human check is labeled `Codex configuration ~/.codex/config.toml`
- **AND** the JSON check keeps its name `codex:target-1` and carries
  `~/.codex/config.toml` as `target`.

#### Scenario: A Codex target lies outside the home directory

- **WHEN** a Codex binding's target lies outside the home directory
- **THEN** the human check is labeled by its position and says that it lies
  outside the home directory
- **AND** neither human nor JSON output contains the file's path.
