# Spec Delta

## REMOVED Requirements

### Requirement: Native artifact acceptance has its own execution closure

**Reason**: Its transport-release scenario describes the GitHub input carrier, which 0.7.0 retires.
**Migration**: "Native artifact acceptance keeps its own execution closure" keeps every other scenario.

## ADDED Requirements

### Requirement: Native artifact acceptance keeps its own execution closure

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
