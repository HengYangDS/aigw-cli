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

AIGW SHALL keep `VERSION` as the sole product-version authority and validate it
with strict Semantic Versioning. `CHANGELOG.md` SHALL follow Keep a Changelog
1.1.0 with exactly one leading `Unreleased` section, canonical change
categories, and released sections in strictly descending semantic-version
order. Every local product tag SHALL have exactly one released section. Every
released section SHALL have a local product tag, except for at most one pending
release section that is first, matches the current untagged `VERSION`, and is
newer than every published version. The exact release tag, `VERSION`, first
released section, and `HEAD` SHALL agree before publication. Version syntax and
chronology checks SHALL NOT infer whether a change is breaking; the Change and
release review remain responsible for selecting the SemVer increment from the
public compatibility impact.

AIGW SHALL validate version identity apart from platform trust.
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

#### Scenario: Development continues between releases

- **WHEN** the current `VERSION` is newer than every local product tag
- **THEN** unreleased user-visible changes SHALL remain under `Unreleased`
- **AND** an ordinary development commit SHALL NOT manufacture a dated release.

#### Scenario: A release commit is prepared before its tag

- **WHEN** the current untagged `VERSION` has a dated release section
- **THEN** that section SHALL be the only untagged released section and the first
  section after `Unreleased`
- **AND** every older released section SHALL already correspond to a product tag.

#### Scenario: Release history and Git disagree

- **WHEN** a product tag lacks a released section, a historical released section
  lacks its tag, or the selected tag differs from `VERSION`, the first released
  section, or `HEAD`
- **THEN** the release gate SHALL fail before construction or publication.

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
