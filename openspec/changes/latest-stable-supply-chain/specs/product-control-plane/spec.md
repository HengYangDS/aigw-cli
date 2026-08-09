## MODIFIED Requirements

### Requirement: Latest stable repository-owned supply chain

AIGW SHALL lock the current stable dependency graph selected by its declared Go
toolchain and SHALL keep transitive ownership in the resolver rather than
duplicating it in repository scripts.

#### Scenario: A stable transitive update is available

- **WHEN** the Go resolver reports a newer stable transitive dependency
- **THEN** the repository SHALL update `go.mod` and `go.sum` together
- **AND** the complete native gate SHALL pass before integration
