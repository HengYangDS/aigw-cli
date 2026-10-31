# product-quality Specification

## Purpose

Define the quality graph, its evidence and the quality of repository text that
make AIGW verifiable and portable, without turning presentation preferences or
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
- **AND** missing native model metadata SHALL remain visible without fabricated catalog entries
- **AND** ordinary version, session and progress stderr SHALL NOT become a warning failure.

#### Scenario: Native Codex rejects an inherited temporary-home relationship

- **WHEN** catalog or verification runs under an inherited temporary root
- **THEN** its native child SHALL receive an owned temporary root that does not contain its selected Codex home
- **AND** parent environment and user configuration SHALL remain unchanged
- **AND** native catalog warnings SHALL prevent qualification rather than silently discard diagnostics
- **AND** failed and successful probes SHALL reclaim their exact owned workspace.

#### Scenario: Native rule conformance exceeds its aggregate execution budget

- **WHEN** independent positive and negative Go rule fixtures qualify the same policy
- **THEN** one bounded native batch SHALL execute every isolated fixture package
- **AND** valid fixtures SHALL have no findings and each invalid fixture SHALL retain its intended diagnostic
- **AND** root resolution, format and type-failure contracts SHALL remain independently exercised
- **AND** policy thresholds, native diagnostics and the original package timeout SHALL NOT be weakened.

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

### Requirement: Benchmark acceptance fails only on product diagnostics

Benchmark acceptance SHALL judge the diagnostics of the product's own prepare
and measured commands. The benchmark tool's statistical warnings SHALL stay in
the retained evidence and SHALL NOT fail acceptance.

#### Scenario: The benchmark tool warns about its statistics

- **WHEN** Hyperfine prints a statistical warning and the measured commands
  print no diagnostic
- **THEN** acceptance SHALL pass
- **AND** the raw output SHALL retain the warning.

#### Scenario: A measured command prints a diagnostic

- **WHEN** the product's prepare or measured command prints a diagnostic
- **THEN** acceptance SHALL fail before timing.

### Requirement: Tests keep Mise state private

A test that runs Mise SHALL direct Mise's state directory into the test's own
workspace, so that trust and tracking records for temporary checkouts never
reach the operator's Mise state.

#### Scenario: A test runs Mise in a temporary checkout

- **WHEN** the test runs Mise in its temporary checkout
- **THEN** Mise's tracking record SHALL be written inside the test workspace.
