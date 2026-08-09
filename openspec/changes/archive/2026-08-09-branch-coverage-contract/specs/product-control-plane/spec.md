## MODIFIED Requirements

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner; composition roots SHALL
only assemble those owners. Source gates MUST reject compatibility shims,
forwarding wrappers, alias-only packages, forbidden product references,
unmanaged flat structure, host-dependent policy paths, and statement coverage
of 95 percent or less for any package or the aggregate. Coverage claims MUST
name their executable measure accurately and MUST NOT infer branch evidence
from a Go statement profile.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change introduces a forbidden owner shape or lowers any package or
  aggregate coverage to 95 percent or less
- **THEN** local and hosted verification SHALL fail before publication

#### Scenario: Foreign-host absolute path enters policy

- **WHEN** policy contains an absolute or parent-traversing path in another
  host's syntax
- **THEN** validation SHALL reject it identically on macOS, Linux, and Windows

#### Scenario: No admitted branch authority exists

- **WHEN** no stable tool can measure the complete module once on every
  supported platform
- **THEN** release evidence names only statement coverage and makes no branch
  claim
