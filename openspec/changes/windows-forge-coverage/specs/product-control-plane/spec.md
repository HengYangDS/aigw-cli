## ADDED Requirements

### Requirement: Native cross-platform release admission

AIGW SHALL require native source verification on Windows, Linux, and macOS
before a trusted release. Every production package and the module aggregate
SHALL maintain statement and branch coverage strictly greater than 95 percent
under the single repository coverage policy, without platform exclusions or
duplicated test stacks.

#### Scenario: A platform exposes an uncovered Forge failure boundary

- **WHEN** one native platform reports a production package below the coverage
  floor
- **THEN** a portable regression SHALL exercise a real behavior or failure
  boundary
- **AND** the coverage policy SHALL remain unchanged
- **AND** all native platforms SHALL execute the same source test suite.
