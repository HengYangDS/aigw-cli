## MODIFIED Requirements

### Requirement: Coverage claims are executable and truthful

AIGW MUST require aggregate statement coverage and every production package's
statement coverage to each be strictly greater than 95 percent. It MUST name
the executable measure accurately and MUST NOT infer branch evidence from a Go
statement profile.

#### Scenario: One measure reaches only the floor

- **WHEN** aggregate statements or any production package are at or below 95
  percent
- **THEN** the single coverage gate fails and identifies the failed measure.

#### Scenario: No admitted branch authority exists

- **WHEN** no stable tool can measure the complete module once on every
  supported platform
- **THEN** release evidence names only statement coverage and makes no branch
  claim.
