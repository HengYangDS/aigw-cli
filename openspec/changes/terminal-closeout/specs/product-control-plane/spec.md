## MODIFIED Requirements

### Requirement: Terminal local release readiness

AIGW SHALL admit a local release candidate only when canonical specifications
contain no placeholder authority, every direct repository dependency is current
and stable, transitive selection remains owned by those direct dependencies,
every package and aggregate statement coverage remain strictly greater than 95
percent, the native source gate passes, and the complete release matrix is
reproducible and installable. Hosted CI, Forge publication, released-asset
installation, and lane retirement SHALL consume the archived result rather than
become prerequisites of the Change that produces it.

#### Scenario: A stable direct dependency update is available

- **WHEN** the declared Go toolchain reports a newer stable direct module version
- **THEN** `go.mod` and `go.sum` SHALL be refreshed together
- **AND** the complete native source gate SHALL pass before integration

#### Scenario: Only an unneeded transitive update is reported

- **WHEN** the module query reports a newer transitive version but `go mod why`
  shows that the main module does not need that module
- **THEN** AIGW SHALL leave selection with the direct dependency owner
- **AND** SHALL NOT add an explicit pin merely to display the newest version

#### Scenario: A canonical document contains placeholder authority

- **WHEN** a specification purpose remains `TBD` or describes generation history
- **THEN** terminal closeout SHALL fail until the purpose states current product
  semantics directly

#### Scenario: Protected branches are projected

- **WHEN** the operator selects `main` for a Forge identity projection
- **THEN** every `main` and `dev` precondition SHALL pass before publication
- **AND** one atomic push SHALL advance both protected branches or neither
- **AND** only an explicit `proposal/*` selection MAY use single-branch projection
- **AND** candidate, work, or arbitrary branches SHALL be rejected.

#### Scenario: External delivery follows local readiness

- **WHEN** the Change has passed exact-HEAD proof and has been archived and landed
- **THEN** native macOS, Linux, and Windows hosted verification MAY consume that
  exact accepted result
- **AND** GitLab and GitHub MAY publish it independently after their own gates
- **AND** released-asset installation and governed lane retirement occur only
  after the corresponding external evidence exists.
