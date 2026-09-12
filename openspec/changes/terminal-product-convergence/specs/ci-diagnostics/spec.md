## ADDED Requirements

### Requirement: One CI graph projects to independent Forges

The repository SHALL define its semantic CI graph once and deterministically
project it to GitHub and GitLab. Each projection SHALL preserve the same named
facts while expressing only genuine provider syntax and runner-capability
differences.

#### Scenario: A projection is regenerated

- **WHEN** the canonical CI graph changes
- **THEN** both Forge configurations are regenerated in one operation
- **AND** validation rejects hand-edited semantic drift.

#### Scenario: One Forge lacks a native runner

- **WHEN** a required native-platform fact is proven on the other admitted
  Forge for the exact product commit
- **THEN** the incapable Forge may omit that executor explicitly
- **AND** parity reports the capability assignment rather than a false missing
  mirror job.

### Requirement: Every integration path produces exact-commit evidence

Proposal creation and update, review commits, maintainer integration, `dev`,
`main`, and release tags SHALL trigger the evidence appropriate to the event
and exact Git object. Evidence MAY be reused only when its inputs and claimed
facts are identical.

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

#### Scenario: A release tag is created

- **WHEN** a signed release tag points to an accepted commit
- **THEN** release construction and publication consume that exact commit
- **AND** `main`, `dev`, the tag, assets, checksums, signatures, provenance, and
  reported version agree where the release policy requires identity.

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
