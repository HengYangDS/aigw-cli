## MODIFIED Requirements

### Requirement: Terminal local release readiness

AIGW SHALL advance a release candidate to current stable releases across the
application, test, and declared repository-tool closure before freezing that
candidate. `go.mod` and `go.sum` SHALL own the compiled Go closure;
`mise.toml` and `mise.lock` SHALL own language runtimes and standalone tools;
`package.json` and `package-lock.json` SHALL own direct and transitive npm
repository tools. A clean runner MUST NOT select an npm transitive dependency
outside the committed lock. Aggregate statement and branch coverage SHALL
remain strictly greater than 95 percent; every package SHALL remain present,
executed, and reported. The native source and release gates SHALL pass before
publication.

#### Scenario: Stable dependency updates are available

- **WHEN** the application, test, or declared repository-tool closure reports newer stable releases
- **THEN** AIGW SHALL refresh the owning ecosystem declaration and lock together
- **AND** SHALL run the complete source, coverage, and release gates
- **AND** SHALL NOT preserve the older graph as a compatibility target

#### Scenario: A clean runner materializes npm tools

- **WHEN** source verification starts without an existing npm installation
- **THEN** the runner SHALL install the exact committed npm dependency graph
- **AND** install scripts SHALL remain disabled
- **AND** no direct or transitive npm version SHALL be selected outside `package-lock.json`
- **AND** registry signatures SHALL verify through the ecosystem's standard verifier

#### Scenario: The verified release is published

- **WHEN** exact-HEAD proof passes for the refreshed source tree
- **THEN** GitLab and GitHub MAY construct their own signed commit and tag provenance
- **AND** both Forge histories SHALL represent the same verified source tree
- **AND** each Forge SHALL publish its complete release asset matrix independently
