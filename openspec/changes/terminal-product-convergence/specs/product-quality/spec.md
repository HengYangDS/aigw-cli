# Spec Delta

## MODIFIED Requirements

### Requirement: Warnings are owned failures

Every repository-owned warning and nonempty native validation finding emitted
by a supported build, test, analysis, documentation, packaging, or CI path
SHALL be resolved at its semantic owner. A validator's advisory severity does
not waive this repository's clean-evidence policy. Successful subprocess exit
SHALL NOT erase captured diagnostic output or establish warning-free acceptance.

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

### Requirement: Native artifact acceptance has its own execution closure

An explicitly selected prebuilt candidate or tagged product SHALL execute the
existing release journey with only its required native tools when full source
quality and lock refresh are not selected. Ordinary source, review and full
qualification SHALL retain complete locked bootstrap, quality and security
checks. Artifact acceptance SHALL NOT substitute for failed source qualification.
A same-version verifier MAY select an exact signed untagged product commit;
matrix signatures, clean-source and provenance admission SHALL remain required.

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

### Requirement: Markdown spacing preserves semantic blocks

Current Markdown SHALL separate adjacent headings, prose, lists, tables and
fenced blocks with one blank line. Lists whose peer items each contain only one
paragraph SHALL have no blank separators between those items. Wrapped lines
SHALL remain part of their paragraph. Native parsing SHALL preserve required
separation within complex items and literal code; immutable archives SHALL
remain unchanged.

#### Scenario: Single-paragraph peer items contain blank separators

- **WHEN** a current list or task list inserts blank lines between peer items
  that each contain only one paragraph
- **THEN** the native Markdown gate SHALL reject that spacing
- **AND** nested and quoted lists SHALL follow the same paragraph-level rule.

#### Scenario: List items contain distinct semantic blocks

- **WHEN** a list contains multiple paragraphs, a nested block, a table or a
  fenced example
- **THEN** the gate SHALL preserve valid blank-line separation
- **AND** literal code SHALL remain outside paragraph-spacing checks.

#### Scenario: Adjacent blocks lack their required separation

- **WHEN** headings, lists, tables or fenced blocks lack required blank lines,
  or prose contains consecutive extra blank lines
- **THEN** the existing native block-spacing rules SHALL reject the defect
- **AND** the read-only gate SHALL leave source bytes unchanged.
