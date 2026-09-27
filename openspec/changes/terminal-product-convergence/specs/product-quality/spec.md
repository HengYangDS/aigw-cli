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
