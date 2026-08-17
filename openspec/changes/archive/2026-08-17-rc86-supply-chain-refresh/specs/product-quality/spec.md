## ADDED Requirements

### Requirement: Terminal local release readiness

AIGW SHALL advance a release candidate to current stable releases across the
application, test, and declared Go tool closure before freezing that candidate.
`go.mod` and `go.sum` SHALL remain the sole dependency selection authority;
modules outside the compiled package and tool closure are not selected supply
chain inputs. Every package and aggregate statement and
branch coverage SHALL remain strictly greater than 95 percent, and the native
source and release gates SHALL pass before publication.

#### Scenario: Stable dependency updates are available

- **WHEN** the compiled application, test, or declared tool closure reports newer stable releases
- **THEN** AIGW SHALL refresh `go.mod` and `go.sum` together
- **AND** SHALL run the complete source, coverage, and release gates
- **AND** SHALL NOT preserve the older graph as a compatibility target

#### Scenario: The verified release is published

- **WHEN** exact-HEAD proof passes for the refreshed source tree
- **THEN** GitLab and GitHub MAY construct their own signed commit and tag provenance
- **AND** both Forge histories SHALL represent the same verified source tree
- **AND** each Forge SHALL publish its complete release asset matrix independently
