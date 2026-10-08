# Spec Delta

## MODIFIED Requirements

### Requirement: Warnings are owned failures

Repository-owned source and configuration warnings SHALL block qualification
until resolved at their semantic owner. Supply-chain security findings SHALL
remain nonblocking and fully disclosed under the
[release evidence contract](../../../../../docs/governance/change-and-release-policy.md#reproducible-assets).
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

### Requirement: Dependency evidence binds the selected lockfiles

The release scanner invocation and report admission SHALL share one exact
lockfile-path selection. Normalization SHALL admit every selected lockfile
exactly once with nonempty package observations and established license
identities before writing either report. An exact version-bound OSV license
override MAY correct a verified upstream metadata gap without suppressing
vulnerability observations.

Dependency scanning and report admission SHALL precede artifact construction.
Scanner failure, malformed output or incomplete scope SHALL NOT launch
GoReleaser, SBOM generation or signing; accepted output and cleanup obligations
SHALL remain unchanged.

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

## ADDED Requirements

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
