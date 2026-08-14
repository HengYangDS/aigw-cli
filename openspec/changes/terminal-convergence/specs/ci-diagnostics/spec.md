## MODIFIED Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted job that runs Git-aware tooling SHALL initialize and verify the
checkout explicitly in its own provider environment. Runner names, platform
labels, and workspace paths SHALL follow the repository's declared project and
platform contract rather than a personal host layout.

#### Scenario: A hosted action initializes a repository

- **WHEN** GitLab or GitHub starts a required job
- **THEN** the job verifies its exact checkout, revision, platform, and toolchain before running gates
- **AND** it neither assumes the peer Forge nor copies the peer's workspace path.

#### Scenario: A required runner is unavailable

- **WHEN** no admitted runner can execute a required platform gate
- **THEN** the pipeline fails or reports an explicit unavailable gate within a bounded interval
- **AND** does not remain pending indefinitely.
