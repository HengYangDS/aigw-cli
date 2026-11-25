## MODIFIED Requirements

### Requirement: One CI graph projects to independent Forges

The repository SHALL define its semantic CI graph once and deterministically
project it to GitHub and GitLab. Each selected Forge SHALL run its own required
macOS and Linux native jobs for the exact product commit. Projection
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

#### Scenario: Transport inputs accompany source qualification

- **WHEN** a native source job supplies only peer and repository locators
- **THEN** it SHALL execute its source lifecycle once
- **AND** transport locators SHALL NOT select a second release acceptance
- **AND** explicit artifact, published-predecessor, client and performance scopes
  SHALL remain independently selected and verified.

#### Scenario: A maintainer selects native performance

- **WHEN** either Forge's manual pipeline selects an exact signed candidate and
  published predecessor for performance measurement
- **THEN** the existing native executor SHALL select the release-owned
  measurement scope with its complete platform tool closure
- **AND** unselected jobs SHALL NOT add performance-only prerequisites
- **AND** raw samples and diagnostics SHALL remain in the existing job artifacts
  on success and failure
- **AND** performance SHALL NOT substitute for required lifecycle evidence.

#### Scenario: A fork requests review execution in the parent project

- **WHEN** a GitLab merge request names a source project other than the project
  executing the parent pipeline
- **THEN** neither the workflow nor its review jobs admit that event onto the
  project's native runners
- **AND** a same-project developer review retains the required quality and
  native checks
- **AND** this fork guard does not establish isolation between same-project
  developer jobs and protected jobs on persistent Shell runners.

#### Scenario: Native review preserves manager and protected authority

- **WHEN** a selected peer admits untrusted review code to a native executor
- **THEN** OS-enforced identity, storage and execution separation SHALL prevent
  that code from reading Runner-manager credentials or protected-job state, or
  elevating into their owner's authority
- **AND** actual job-context denied access and per-job cleanup before reuse SHALL
  be qualified without reading secret values
- **AND** distinct identities, cleared environments and successful source checks
  alone SHALL NOT qualify containment
- **AND** executor destruction MAY implement the boundary but SHALL NOT be the
  required topology.

#### Scenario: A maintainer omits the manual commit base

- **WHEN** either Forge starts manual verification without an explicit commit base
- **THEN** source quality checks the selected checkout and product provenance
  verifies only the selected commit against its direct parent
- **AND** an explicit commit base overrides that default to verify its full
  introduced range
- **AND** the manual result never substitutes for the required review, branch,
  tag, or release event evidence.

#### Scenario: Release assets are verified on a peer

- **WHEN** a selected Forge verifies a published release's assets
- **THEN** its release verification depends on its own quality, version, and
  complete native matrix for the exact release object
- **AND** no cross-peer download or result is an input.

#### Scenario: GitLab verifies release assets after native jobs

- **WHEN** GitLab downloads and verifies the complete released artifact matrix
- **THEN** its OS-independent verifier runs on the Linux container selector,
  not the persistent macOS Shell selector that also accepts review code
- **AND** macOS native evidence remains required on its own job
- **AND** this relocation does not by itself prove Shell runner isolation for
  review, tag, or other protected work.

#### Scenario: The sibling product peer is unavailable

- **WHEN** a selected AIGW Git repository, Release, and API are unavailable
- **THEN** the other selected peer SHALL run its required graph from its own
  exact product object without fetching AIGW source, policy, evidence, or
  assets from the unavailable peer
- **AND** tool distribution hosted by the unavailable Forge platform SHALL
  NOT be treated as independent or waived by disclosure.

#### Scenario: Cold-cache CI survives a sibling platform outage

- **WHEN** one selected Forge platform and its container and asset endpoints
  are unavailable, the other peer has no restored tool cache, and a new product
  SHA requires its complete CI graph
- **THEN** the other peer SHALL reconstruct its locked tool closure without
  that unavailable platform and execute its required macOS and Linux
  jobs for that exact SHA
- **AND** a warm cache, another peer's result, or a partial matrix SHALL NOT
  substitute for this cold-cache evidence
- **AND** a missing runner or tool SHALL leave that peer unverified, not green
  by exception.
