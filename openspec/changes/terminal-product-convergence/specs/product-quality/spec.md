# Spec Delta

## MODIFIED Requirements

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
