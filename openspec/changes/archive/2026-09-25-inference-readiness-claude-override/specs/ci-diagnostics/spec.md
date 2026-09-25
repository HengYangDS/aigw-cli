## MODIFIED Requirements

### Requirement: One CI graph projects to independent Forges

The repository SHALL define its semantic CI graph once and deterministically
project it to GitHub and GitLab. Each selected Forge SHALL run its own required
macOS, Linux, and Windows native jobs for the exact product commit. Projection
differences SHALL be limited to provider syntax, runner selectors, and
platform-specific shell commands. Neither Forge's result SHALL substitute for
a required job on the other Forge.

#### Scenario: A projection is regenerated

- **WHEN** the canonical CI graph changes
- **THEN** both Forge configurations are regenerated in one operation
- **AND** validation rejects hand-edited semantic drift.

#### Scenario: One Forge lacks a native runner

- **WHEN** a selected Forge cannot execute a required native-platform job
- **THEN** that job remains required and that Forge is not accepted as green
- **AND** the other Forge may continue independently, but its result does not
  satisfy the missing peer-local job.

#### Scenario: A maintainer selects one platform for diagnosis

- **WHEN** manual verification selects one native platform
- **THEN** the selected peer MAY execute only that platform's native job
- **AND** the partial result does not satisfy complete review, accepted-branch,
  tag, or release readiness.

#### Scenario: Release assets are verified on a peer

- **WHEN** a selected Forge verifies a published release's assets
- **THEN** its release verification depends on its own quality, version, and
  complete native matrix for the exact release object
- **AND** no cross-peer download or result is an input.

#### Scenario: The sibling product peer is unavailable

- **WHEN** a selected AIGW Git repository, Release, and API are unavailable
- **THEN** the other selected peer SHALL run its required graph from its own
  exact product object without fetching AIGW source, policy, evidence, or
  assets from the unavailable peer
- **AND** third-party tool distribution remains a separately disclosed
  availability boundary, not peer-local product evidence.
