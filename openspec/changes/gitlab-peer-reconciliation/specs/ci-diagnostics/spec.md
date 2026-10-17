# Spec Delta

## ADDED Requirements

### Requirement: Native review isolation proves denial of its protected resource

A native review job given an exact protected resource SHALL report isolation
only when its own identity cannot read that resource and holds no elevation
grant. A resource behind a directory that denies the review identity traversal
SHALL count as unreadable, and the report SHALL mark its identity as asserted
by the operator, because the review identity cannot observe it.

#### Scenario: The resource is visible but unreadable

- **WHEN** the review identity can observe the resource as a regular file but
  opening it is denied, and the noninteractive sudo listing shows no grant
- **THEN** the job SHALL report a denied direct read and no elevation grant.

#### Scenario: The resource sits behind a closed directory

- **WHEN** observing the resource is denied by permission
- **THEN** the job SHALL still require the absence of an elevation grant
- **AND** it SHALL report a denied direct read at traversal with an
  operator-asserted identity.

#### Scenario: Isolation cannot be shown

- **WHEN** the resource is readable, missing from a visible directory or not a
  regular file, or sudo reports a grant or cannot prove its absence
- **THEN** the job SHALL fail before product execution.
