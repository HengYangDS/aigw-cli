## MODIFIED Requirements

### Requirement: actor-independent contribution policy

The repository SHALL require structured commit messages and trusted signatures
without binding a personal name, email, key, fingerprint, host path, signing
program, or Forge credential in source. Product identity and trust SHALL be
explicit publication inputs; each peer SHALL independently supply only its
transport credential and hosted account verification.

#### Scenario: an admitted team contributor commits

- **WHEN** a commit enters protected product history
- **THEN** its unchanged message, identities, and signature SHALL satisfy the
  explicit product trust policy.

#### Scenario: one Forge is unavailable

- **WHEN** GitLab or GitHub cannot verify or publish
- **THEN** local verification and the other peer SHALL remain independently
  executable and SHALL NOT claim success for the unavailable peer.

### Requirement: Independent Forge parity

GitLab and GitHub SHALL be independent projections of one local Git object
authority. For every newly published product branch and formal release, local
Git and each selected peer SHALL expose the exact same commit OID, annotated tag
object OID, peeled commit, and tree. Tree-only equality, provider-qualified tag
namespaces, identity replay, and commit maps SHALL NOT be accepted as parity.

#### Scenario: Equivalent provider projection

- **WHEN** one signed local commit or annotated tag is published to both peers
- **THEN** the complete object identity SHALL be exactly equal on local Git,
  GitLab, and GitHub.

#### Scenario: Real source drift

- **WHEN** a peer commit or tag has an equal tree but a different object OID
- **THEN** synchronization SHALL fail as real product-object drift.

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent peer publication, exact
branch and tag identity, asset integrity, installation, runtime acceptance, and
repository housekeeping. A release SHALL be complete only when its one signed
tag object, immutable assets, checksums, peer-native Release records, and
supported-platform acceptance are verified independently on each declared
publication plane.

#### Scenario: both publication planes complete

- **WHEN** GitLab and GitHub independently publish one accepted product release
- **THEN** their commit and annotated tag object identifiers SHALL equal local
  Git exactly
- **AND** their asset manifests and supported-platform semantics SHALL agree.

#### Scenario: local proof passes but delivery is incomplete

- **WHEN** hosted CI, a selected peer, exact object identity, asset integrity,
  installation, runtime acceptance, or lane retirement remains unverified
- **THEN** the repository SHALL report that stage as incomplete and SHALL NOT
  claim terminal completion.

#### Scenario: terminal closeout succeeds

- **WHEN** every delivery stage passes for the exact accepted product object
  and obsolete lanes, policies, compatibility paths, temporary assets, and
  stale runtime residue are retired
- **THEN** the repository MAY report completion with receipts for each
  independent boundary.

#### Scenario: release metadata exists without publication

- **WHEN** `VERSION` and `CHANGELOG` name a release but either selected peer
  lacks its exact signed tag object, Release record, or assets
- **THEN** delivery SHALL remain incomplete.
