# product-quality Specification

## Purpose

Define the quality graph, its evidence and the quality of repository text that
make AIGW verifiable and portable, without turning presentation preferences
into arbitrary merge vetoes.

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

- **WHEN** OpenSpec reports `INFO`, `WARNING`, `ERROR`, or a failed summary,
  other than its acknowledgement of a Change's own `skip_specs` declaration
- **THEN** the repository gate fails and reports every finding without filtering
  it, even if the native validator classifies the item as informational
- **AND** unknown severity, malformed evidence, or diagnostic output failure
  also fails admission.

#### Scenario: A Change declares that it changes no specification

- **WHEN** an active Change declares `skip_specs: true`, holds no delta, and
  OpenSpec's only finding for it is the acknowledgement of that declaration
- **THEN** the repository gate passes and names that Change in its summary as
  one without a specification change
- **AND** any other finding of that Change still fails the gate
- **AND** the same acknowledgement for a Change whose metadata does not declare
  `skip_specs: true` fails the gate.

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
- **THEN** public verification SHALL report incomplete qualification in its own words
- **AND** for Codex and Claude it SHALL show the client's text only on request, bounded, and only for a session
  whose credential AIGW holds and removes from that text
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
reach the operator's Mise state. It SHALL keep the data and cache directories
that its caller selected and SHALL NOT install a tool, so the caller's
installed tools stay in use and the caller's tool store stays unchanged.

#### Scenario: A test runs Mise in a temporary checkout

- **WHEN** the test runs Mise in its temporary checkout
- **THEN** Mise's tracking record SHALL be written inside the test workspace.

#### Scenario: A hosted job selects Mise's tool directories

- **WHEN** a hosted job directs Mise's data and cache directories into
  job-owned directories, as GitLab's Linux and Windows jobs do
- **THEN** a test that isolates Mise's configuration SHALL run the tools
  installed there and install none.

### Requirement: Native tests pass on every hosted platform

The repository's tests SHALL pass on every host whose native job runs them,
including GitLab's Linux, Windows and macOS ARM64 hosts. A test of
host-specific behavior SHALL verify the outcome that the specification
declares for the host where it runs. A bound on a helper process SHALL stop a
hang, not a slow host.

#### Scenario: A host has no declared client layout

- **WHEN** a native client supply test runs on a host for which the repository
  declares no official client layout, such as GitLab's Linux ARM64 runner
- **THEN** it SHALL verify that the supply refuses the host before it creates
  a supply directory or exports a client
- **AND** a test that needs the supply's installers to start SHALL skip on that
  host and name the refusal.

#### Scenario: Windows briefly holds a readiness record

- **WHEN** a test reads the record that a helper process published to signal
  its readiness on Windows, and the host still holds the file
- **THEN** the read SHALL retry within a bounded time before the test fails.

#### Scenario: A loaded host slows a helper process

- **WHEN** a helper process that a test starts completes its work later on a
  loaded host than on an idle one
- **THEN** the test SHALL accept the helper's result within a bound that stops
  only a hang.

### Requirement: Hang bounds tolerate a shared CI host

Every bound that a test or the coverage run uses to stop a hang SHALL leave room for a native CI host three times slower than a quiet one; each package run SHALL use the limit that the coverage policy declares, and an interruption test SHALL count only an interruption that lands after its target has started.

#### Scenario: A shared host runs the suite slowly

- **WHEN** a native job shares its CI host and its packages run three times slower than on a quiet host
- **THEN** no package SHALL reach the policy's run limit, and no hang bound SHALL stop a test that is still making progress.

#### Scenario: An interruption lands before the work it targets

- **WHEN** a test interrupts a native preflight to prove cleanup, and the interruption lands before the workload has started
- **THEN** the test SHALL repeat it with a longer window, and every attempt SHALL still prove that the processes it observed exited within its cleanup bound.

#### Scenario: A process hangs

- **WHEN** a selected child test or a package stops making progress
- **THEN** its bound SHALL stop it and fail the run with Go's diagnostics before the job's own time budget ends.

#### Scenario: The coverage policy omits or misstates the package limit

- **WHEN** the coverage policy's package limit is missing, unparsable or not positive
- **THEN** the coverage run SHALL refuse the policy before it runs any test.
