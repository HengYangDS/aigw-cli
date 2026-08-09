## MODIFIED Requirements

### Requirement: Coverage claims are executable and truthful

AIGW MUST require aggregate statement coverage, every production package's
statement coverage, and aggregate branch coverage to each be strictly greater
than 95 percent. Statement and branch measurements MUST retain distinct named
authorities and MUST NOT be inferred from one another.

#### Scenario: One measure reaches only the floor

- **WHEN** aggregate statements, any production package, or aggregate branches
  are at or below 95 percent
- **THEN** the single coverage gate fails and identifies the failed measure.

#### Scenario: Branch evidence is unavailable

- **WHEN** the branch instrumenter cannot enumerate, instrument, execute, or
  produce a parseable summary for any production package
- **THEN** the gate fails closed rather than reporting statement success as
  branch success.
