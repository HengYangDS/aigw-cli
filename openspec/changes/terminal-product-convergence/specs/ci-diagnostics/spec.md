# Spec Delta

## MODIFIED Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted Git-aware job SHALL verify its checkout, revision, platform, and
complete repository-locked executable closure. CUE SHALL select one mise
bootstrap release for both Forges; GitLab Linux SHALL use its digest-pinned
image and shared install step. npm SHALL consume `package-lock.json` with
scripts disabled. Tool authentication SHALL remain independent of product Git
peers. Each selected peer SHALL be able to obtain its locked tool closure from
independent upstreams or peer-controlled immutable assets while the sibling
Forge platform is unavailable. No particular mirror or unrelated peer SHALL
be mandatory.

#### Scenario: A hosted action initializes a repository

- **WHEN** checkout or a test fixture initializes Git state
- **THEN** Git resolves `main` as the default branch
- **AND** the repository-declared runtime and standalone tools are installed
  from their locked distribution sources, without consulting another product
  peer for source, policy, or evidence
- **AND** npm repository tools are installed from the committed transitive lock with install scripts disabled
- **AND** GitLab Linux bootstrap is defined once and inherited by every consuming job
- **AND** GitLab source verification uses only its declared source-tool closure
- **AND** native acceptance can execute every tool in its declared command closure
- **AND** no verification or provenance gate is weakened

#### Scenario: A required runner is unavailable

- **WHEN** no admitted runner can execute a required platform gate
- **THEN** the pipeline fails or reports the unavailable gate within a bounded interval
- **AND** does not remain pending indefinitely

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
- **AND** macOS and Windows native evidence remains required on their own jobs
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
  that unavailable platform and execute its required macOS, Linux, and Windows
  jobs for that exact SHA
- **AND** a warm cache, another peer's result, or a partial matrix SHALL NOT
  substitute for this cold-cache evidence
- **AND** a missing runner or tool SHALL leave that peer unverified, not green
  by exception.
