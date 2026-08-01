## MODIFIED Requirements

### Requirement: Enforced semantic ownership and quality

Each behavior and policy SHALL have one semantic owner; composition roots SHALL
only assemble those owners. Source gates MUST reject compatibility shims,
forwarding wrappers, alias-only packages, forbidden product references,
unmanaged flat structure, and statement coverage of 95 percent or less for any
package or the aggregate. Local verification MUST use controlled fixtures rather
than an undeclared public network dependency. A dated quantitative observation
MUST identify its source revision and tree, retain its numerator and denominator,
and derive rather than independently assert its displayed percentage.

#### Scenario: Architecture or coverage regresses

- **WHEN** a change introduces a forbidden owner shape or lowers any package or
  aggregate coverage to 95 percent or less
- **THEN** local and hosted verification SHALL fail before publication

#### Scenario: A local test depends on undeclared external state

- **WHEN** a verification test would contact a public endpoint instead of its
  controlled fixture
- **THEN** local verification SHALL fail or the fixture SHALL intercept the
  exact request without public network I/O

#### Scenario: Quantitative evidence is incomplete or inconsistent

- **WHEN** dated evidence omits its source identity or raw counts, its percentage
  does not match those counts, or its claim digest does not match the record
- **THEN** governance verification SHALL fail before promotion
