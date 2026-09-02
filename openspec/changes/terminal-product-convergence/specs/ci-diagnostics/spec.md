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

#### Scenario: A release tag is created

- **WHEN** a signed release tag points to an accepted commit
- **THEN** release construction and publication consume that exact commit
- **AND** `main`, `dev`, the tag, assets, checksums, signatures, provenance, and
  reported version agree where the release policy requires identity.
