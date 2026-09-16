## MODIFIED Requirements

### Requirement: complete delivery evidence

Quality completion SHALL require distinct evidence for the complete local
graph, exact-HEAD proof, native hosted CI, independent peer publication, exact
branch and tag identity, asset integrity, installation, runtime acceptance, and
repository housekeeping. A release SHALL be complete only when its one signed
tag object, immutable assets, checksums, peer-native Release records, and
supported-platform acceptance are verified at their owning boundaries. Every
selected peer SHALL verify its own objects and assets; native platform evidence
MAY be supplied by the admitted aggregate executor set without duplicating
unavailable runners or weakening the platform requirement.

macOS release binaries SHALL have an explicitly supplied certificate-bound
identity and Hardened Runtime before archive construction. Offline qualification
signing and archive timestamps SHALL use the release epoch. Production
distribution SHALL separately require an Apple-issued Developer ID identity,
secure timestamp, successful notarization and native artifact verification.
Reproducible payload evidence SHALL remain distinct from externally issued
timestamp and notarization evidence. Selected peers SHALL receive the same
immutable distribution bytes, not independently re-signed artifacts.
Missing signing inputs SHALL stop construction without
prompting, provisioning an identity or substituting ad-hoc signing. Native
acceptance SHALL select only its host operating system; unrelated platform
credentials SHALL NOT be prerequisites. Consuming published assets SHALL NOT
require their private signing keys.

macOS retained-Keychain qualification SHALL exercise partitioned database format
`0x200` and both designated-requirement and application-partition authorization.
A self-signed certificate with a matching designated requirement SHALL NOT count
as proof of stable partition identity across changed code hashes. Release
admission SHALL require an approved Apple-recognized signing identity and an
observed retained-credential transition without prompts or ACL mutation.

#### Scenario: A private Keychain fixture models the system authorization boundary

- **WHEN** native credential tests create an isolated Keychain
- **THEN** they SHALL prove partitioned format and the item's partition ACL
- **AND** a same-byte reader SHALL retain access while a changed self-signed
  code-hash partition SHALL be denied without modifying access policy.

#### Scenario: Offline certificate-signed archives are rebuilt

- **WHEN** offline qualification uses identical source, toolchain, isolated
  signing inputs and release epoch on either side of a wall-clock boundary
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** macOS binaries extracted from the archives SHALL satisfy the declared
  native signature requirement and enable Hardened Runtime
- **AND** that result SHALL NOT establish production timestamping, notarization
  or access to retained credentials.

#### Scenario: Native Linux or Windows acceptance builds its asset

- **WHEN** native acceptance runs without macOS signing inputs
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix.

#### Scenario: Native signing authority is missing

- **WHEN** macOS or complete release construction lacks a required identity,
  password-file or designated-requirement input
- **THEN** it SHALL stop before external build execution
- **AND** existing release output, credentials and host trust SHALL remain intact.

#### Scenario: System Keychain qualification lacks a supplied signing identity

- **WHEN** macOS native acceptance requests system credential-store verification
  without operator-supplied signing inputs
- **THEN** fixture preparation SHALL fail before generating a disposable signing
  identity, constructing an archive or touching the system credential store
- **AND** the diagnostic SHALL identify missing signing authority rather than
  a later credential-read failure
- **AND** supplying inputs SHALL NOT itself establish certificate trust or
  retained-credential authorization.

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
