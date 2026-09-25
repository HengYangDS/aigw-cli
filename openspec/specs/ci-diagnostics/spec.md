# ci-diagnostics Specification

## Purpose

Define quiet, deterministic hosted CI diagnostics that expose actionable
failures without relying on runner-global state.

## Requirements

### Requirement: Hosted Git initialization is explicit

Every hosted Git-aware job SHALL verify its checkout, revision, platform, and
complete repository-locked executable closure. CUE SHALL select one mise
bootstrap release for both Forges; GitLab Linux SHALL use its digest-pinned
image and shared install step. npm SHALL consume `package-lock.json` with
scripts disabled. Tool authentication SHALL remain independent of product Git
peers; no mirror or unrelated peer SHALL be mandatory.

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

### Requirement: Every integration path produces exact-commit evidence

Proposal creation and update, review commits, maintainer integration, `dev`,
`main`, and release tags SHALL trigger the evidence appropriate to the event
and exact Git object. Evidence MAY be reused only when its inputs and claimed
facts are identical and an admitted verifier establishes that equivalence.
Until that verifier exists, each accepted-branch and release-branch push SHALL
execute its own required graph, even when their object IDs are equal.

#### Scenario: A proposal receives another commit

- **WHEN** a contributor updates an open proposal
- **THEN** required review checks run for the new exact commit
- **AND** an earlier green commit cannot satisfy the updated proposal.

#### Scenario: A maintainer advances an accepted branch

- **WHEN** an authorized maintainer uses an admitted fast-forward path
- **THEN** the destination event still obtains or verifies its required
  exact-commit evidence
- **AND** bypassing a merge request does not bypass product proof.

#### Scenario: Integration is reviewed for release

- **WHEN** a review targets the declared release branch on either Forge
- **THEN** the selected review commit SHALL receive source quality and the
  Forge's admitted native checks before merging
- **AND** updating that review SHALL check its new selected commit
- **AND** accepted-ref parity SHALL remain a release-branch push observation,
  not a prerequisite that demands the unmerged target already equal the source.

#### Scenario: A reviewed proposal reaches the accepted branch

- **WHEN** a proposal review merges into the declared accepted branch
- **THEN** its resulting push SHALL run the required graph for that accepted
  object, independently of the earlier review result
- **AND** it SHALL NOT require the release branch to have advanced already.

#### Scenario: A maintainer publishes equal accepted and release refs

- **WHEN** an atomic publication advances both protected refs to one object
- **THEN** each resulting branch event SHALL execute its required verification
- **AND** only the release-branch push SHALL check that both refs name its exact
  object; matching SHAs alone SHALL NOT stand in for verified evidence reuse.

#### Scenario: A release tag is created

- **WHEN** a signed release tag points to an accepted commit
- **THEN** release construction and publication consume that exact commit
- **AND** `main`, `dev`, the tag, assets, checksums, signatures, provenance, and
  reported version agree where the release policy requires identity.

#### Scenario: Published artifacts are ready for hosted observation

- **WHEN** publication and immediate asset readback have completed for a peer
- **THEN** the delivering operator SHALL dispatch that peer's artifact
  verification with the exact release tag
- **AND** the verifier SHALL download from that peer rather than another peer
- **AND** Release record creation alone SHALL NOT imply complete asset upload
- **AND** hosted artifact verification SHALL NOT require a signing secret or
  repeat artifact construction.

### Requirement: Output failure preserves the operation boundary

Required verification and publication commands SHALL propagate output failures
without misrepresenting which effects have already completed.

#### Scenario: A gate cannot report its start

- **WHEN** the shared executor cannot write required progress for the next gate
- **THEN** it reports the output error and does not execute that gate or any later
  gate.

#### Scenario: Publication succeeds but its report cannot be written

- **WHEN** a Forge release has been verified and writing its result fails
- **THEN** the command returns the output error with completed-publication context
- **AND** it neither rolls back nor repeats the completed external operation.

### Requirement: Publication credentials remain within their authority

Metadata and upload requests SHALL use their selected endpoint without automatic
redirects. Download verification MAY follow CDN redirects while preserving HTTPS
and the native redirect bound or a caller-provided stricter policy. Publication
credentials SHALL be scoped to the selected API authority for asset downloads.
The verified Forge response MAY select a distinct upload host; its URL SHALL be
absolute HTTP(S), contain no user information and preserve HTTPS.

#### Scenario: Metadata or upload redirects elsewhere

- **WHEN** a metadata or upload endpoint returns a redirect
- **THEN** publication reports failure without contacting the redirect target
- **AND** no caller-owned HTTP client is modified.

#### Scenario: An asset follows a CDN redirect

- **WHEN** artifact retrieval crosses a scheme, host or effective-port boundary
- **THEN** the next request contains no publication credentials
- **AND** later hops do not restore those credentials, even on return to origin
- **AND** HTTPS downgrade fails before contacting the insecure endpoint.

#### Scenario: Metadata provides a direct external asset link

- **WHEN** a verified release names an asset outside the selected API authority
- **THEN** the asset is fetched without publication credentials
- **AND** its bytes must still match the locally verified artifact.
