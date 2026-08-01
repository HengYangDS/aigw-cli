## MODIFIED Requirements

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner; composition roots SHALL
only assemble those owners. Source gates MUST reject compatibility shims,
forwarding wrappers, alias-only packages, forbidden product references,
unmanaged flat structure, and statement coverage of 95 percent or less for any
package or the aggregate.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change introduces a forbidden owner shape or lowers any package or
  aggregate coverage to 95 percent or less
- **THEN** local and hosted verification SHALL fail before publication

## ADDED Requirements

### Requirement: Documented package ownership

Every non-command production package MUST document its package contract at the
implementation owner.

#### Scenario: Package ownership is undocumented

- **WHEN** a production package omits a `Package <name>` contract or documents
  another package name
- **THEN** architecture verification SHALL fail before publication

### Requirement: Quiet handled failures

Handled CLI failures MUST NOT emit a framework usage banner, warning,
traceback, or false completion message.

#### Scenario: A handled CLI failure occurs

- **WHEN** a command returns an expected operational error
- **THEN** the command SHALL return that error without usage, warning,
  traceback, or completion residue
