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

Upgrade acceptance SHALL retain each enabled client's original credential
command, arguments and process environment and execute that invocation after
program replacement but before synchronization or client configuration reload.
Native-store acceptance SHALL retain the original item and identify the real
reader implementation and authorization identity on both sides. A fresh client
consuming a replacement helper SHALL NOT establish existing-caller continuity.
Complete rollback SHALL restore compatible program, configuration and credential
ownership without requiring a credential fallback or new host helper.

Credential-free construction SHALL retain verified ad-hoc Mach-O signatures
and Hardened Runtime with release-epoch timestamps for native qualification.
Public macOS distribution SHALL additionally verify Developer ID signing and
accepted Apple notarization before final checksums and publication. Trusted
signing timestamps and notarization SHALL be distinguished from reproducible
construction; neither SHALL change the credential reader or its authorization.
Detached SSH manifest signatures, checksums, provenance and complete native
acceptance SHALL remain required. Selected peers SHALL receive the same immutable
artifact bytes. Native acceptance SHALL select only its host operating system;
full construction SHALL emit the complete target matrix. Consuming published
assets SHALL NOT require their private signing keys or developer membership.

macOS retained-Keychain qualification SHALL preserve the published go-keyring
`/usr/bin/security` provider across predecessor, candidate and rollback. It SHALL
execute each retained original client credential command before synchronization,
return the same Token and preserve configuration bytes without ACL mutation,
backend migration or a replacement helper. Artifact signing and public notarization remain independent distribution
requirements. Neither SHALL be a prerequisite for routine credential access.

Ordinary source tests SHALL use provider doubles and SHALL NOT touch the host
credential store. Every real Keychain journey SHALL require explicit disposable-
host scope before native access, use exact owned slots and verify their removal.
The scope declaration SHALL NOT be represented as a sandbox or proof that the
machine is disposable. Source-only success SHALL NOT substitute for released-
artifact credential evidence.

Manual native qualification MAY select one supported platform through the
existing CUE-owned Forge projections. Omitted or `all` selection SHALL preserve
the complete available platform set. Selection SHALL NOT remove source quality
or narrow review, accepted-branch push or tag admission. A partial manual result
SHALL prove only its selected platform, not complete release readiness or
required review admission. A Forge's unavailable runner SHALL remain an explicit
capacity boundary rather than an inferred product pass.

#### Scenario: A maintainer qualifies an updated Windows toolchain

- **WHEN** manual verification explicitly selects Windows and full quality
- **THEN** the Windows native job SHALL run its existing complete quality,
  source and packaged lifecycle commands without launching unrelated native jobs
- **AND** source quality SHALL remain selected and other required platforms
  SHALL retain their independent evidence obligations.

#### Scenario: A platform selector is supplied during release or review admission

- **WHEN** a tag, review or accepted-branch push triggers verification
- **THEN** its complete available native set SHALL remain selected
- **AND** manual qualification SHALL NOT replace missing review checks.

#### Scenario: A user consumes the publisher's signed release

- **WHEN** a user installs and invokes published AIGW
- **THEN** no developer membership or private signing key SHALL be required
- **AND** the exact reader's native credential permission SHALL remain a separate
  acceptance obligation from the artifact signature and distribution trust.

#### Scenario: A proposed adapter survives only CLI-only updates

- **WHEN** unchanged adapter bytes retain item access while the CLI changes
- **THEN** actual adapter replacement, rollback and caller authorization SHALL
  remain unproved until their own retained-item journeys pass
- **AND** preserving a vulnerable old reader SHALL NOT satisfy safe updates.

#### Scenario: A client retains its credential command across an update

- **GIVEN** a client has already loaded a valid credential command and environment
- **WHEN** the installed AIGW program is replaced without changing its route
- **THEN** that retained invocation SHALL return the original authorized Token
  before synchronization or client restart
- **AND** updated projections or success from a new helper SHALL NOT substitute
  for that invocation
- **AND** native credential denial SHALL block production acceptance rather than
  trigger an alternate reader, ACL change or repeated authorization attempt.

#### Scenario: Ordinary source verification isolates native credentials

- **WHEN** source verification exercises the macOS credential worker
- **THEN** it SHALL use the injected provider boundary without accessing the
  operator Keychain
- **AND** this result SHALL NOT qualify retained system credentials.

#### Scenario: Explicit Keychain qualification has no disposable-host scope

- **WHEN** macOS native qualification enables system credentials without
  `AIGW_SYSTEM_CREDENTIAL_TEST_SCOPE=ephemeral-host`
- **THEN** it SHALL fail before executing any quality, test or release command
- **AND** directly selected integration tests SHALL reject that scope before
  native credential access.

#### Scenario: Explicit Keychain qualification uses a disposable host

- **WHEN** macOS native qualification selects both system credentials and the
  disposable-host scope
- **THEN** the existing coverage invocation SHALL include integration tests once
- **AND** selecting full quality SHALL preserve that same inclusion and scope.

#### Scenario: Internal archives are rebuilt without publisher credentials

- **WHEN** construction uses identical source, toolchain and release epoch on
  either side of a wall-clock boundary without publisher credentials
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** extracted macOS binaries SHALL pass native signature verification
  with ad-hoc signatures and Hardened Runtime
- **AND** that result SHALL NOT establish publisher trust, notarization or
  access to retained credentials.

#### Scenario: Native acceptance builds its asset

- **WHEN** native acceptance runs on macOS, Linux or Windows without Apple
  signing credentials
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix
- **AND** construction SHALL leave existing credentials and host trust intact.

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
