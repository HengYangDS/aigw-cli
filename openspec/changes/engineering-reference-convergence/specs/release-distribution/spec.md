# Spec Delta

## ADDED Requirements

### Requirement: Stable publication follows completed change acceptance

AIGW SHALL complete and archive the relevant OpenSpec Change before publishing
its stable release. Pre-publication acceptance SHALL consume immutable
candidate artifacts without requiring a public stable tag. The release
procedure SHALL verify the final source and artifact identities, selected-peer
downloads, package-manager projection, and installed behavior after archival.
Archive completion SHALL NOT by itself establish distribution success.

#### Scenario: Implementation or client acceptance remains incomplete

- **WHEN** a required Change task, requested Adapter, or candidate journey remains open
- **THEN** stable tagging and publication SHALL remain pending
- **AND** review branches and isolated candidate validation MAY continue.

#### Scenario: Completed source proceeds to distribution

- **WHEN** the completed Change has been archived and its final source is accepted
- **THEN** the release procedure SHALL qualify the exact final artifact inputs
- **AND** every selected peer and package-manager projection SHALL preserve those bytes
- **AND** published download and installed-product observations remain separate evidence.

## MODIFIED Requirements

### Requirement: Stable identity and platform trust are distinct

AIGW SHALL validate strict semantic versions apart from platform trust.
Credential-free macOS builds retain ad-hoc signatures, Hardened Runtime, and
release-epoch timestamps. Public macOS distribution requires Developer ID
signing and accepted notarization before publication; Windows Authenticode
status remains explicit. Reproducibility, publisher trust, notarization, and
credential authorization are separate claims. Consumers require no publisher
secret or developer membership.

#### Scenario: A version parses but distribution evidence is missing

- **WHEN** a valid stable version lacks required product or channel evidence
- **THEN** distribution SHALL stop with the specific missing evidence
- **AND** version parsing alone SHALL NOT claim release readiness.

#### Scenario: A user consumes the publisher's signed release

- **WHEN** a user installs and invokes published AIGW
- **THEN** no developer membership or private signing key SHALL be required
- **AND** the exact reader's native credential permission SHALL remain separate
  from artifact signature and distribution trust.

#### Scenario: Release metadata exists without publication

- **WHEN** `VERSION` and `CHANGELOG` name a release but either selected peer
  lacks its exact signed tag object, Release record, or assets
- **THEN** delivery SHALL remain incomplete.

### Requirement: Final distribution bytes have one authority

Signing and notarization SHALL precede final inventory and checksums. Detached
SSH manifest signatures, provenance, and native acceptance remain required. Each
selected Forge and package-manager projection SHALL identify the same accepted
bytes. Native acceptance builds host targets; full construction emits the
complete matrix. Reproducibility remains separate from trusted timestamps and
notarization. Published bytes SHALL NOT be replaced under an existing release
identity.

#### Scenario: A published release already exists

- **WHEN** a newly signed artifact differs from a published artifact
- **THEN** it SHALL require a new release identity rather than replacing the published bytes.

#### Scenario: Internal archives are rebuilt without publisher credentials

- **WHEN** construction uses identical source, toolchain, and release epoch on
  either side of a wall-clock boundary without publisher credentials
- **THEN** the complete archive matrix SHALL be byte-identical
- **AND** extracted macOS binaries SHALL pass native signature verification
  with ad-hoc signatures and Hardened Runtime
- **AND** that result SHALL NOT establish publisher trust, notarization, or
  access to retained credentials.

#### Scenario: Native acceptance builds its asset

- **WHEN** native acceptance runs on macOS, Linux, or Windows without Apple
  signing credentials
- **THEN** it SHALL build only its current operating system's declared targets
- **AND** full release construction SHALL still require the complete matrix
- **AND** construction SHALL leave existing credentials and host trust intact.

#### Scenario: Selected peers expose identical release assets

- **WHEN** GitLab and GitHub publish one accepted product release
- **THEN** their immutable asset bytes and manifests SHALL agree
- **AND** their supported-platform semantics SHALL be equivalent.
