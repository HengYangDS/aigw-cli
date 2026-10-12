## MODIFIED Requirements

### Requirement: Stable identity and platform trust are distinct

`VERSION` SHALL own strict SemVer identity. `CHANGELOG.md` SHALL follow Keep a
Changelog 1.1.0 with one leading `Unreleased` and descending canonical releases.
It SHALL document reachable tags, bind history to allocated versions and permit
one current pending release. Publication SHALL bind tag, VERSION, first release
and HEAD. Identity, reproducibility, publisher trust, notarization and credential
authorization SHALL remain separate claims.

#### Scenario: Version history has one bounded pending release

- **WHEN** AIGW validates local product versions and Changelog history
- **THEN** every product tag reachable from `HEAD` SHALL have exactly one released
  section and every released section SHALL have an allocated product tag, except
  for at most one pending section
- **AND** the pending section SHALL be first after `Unreleased`, match the current
  untagged `VERSION` and be newer than every locally allocated product version.

#### Scenario: A maintenance release starts from an older published source

- **WHEN** a maintenance source does not inherit another locally tagged release
- **THEN** its Changelog SHALL preserve reachable release history and already
  recorded product chronology without requiring unmerged release content
- **AND** strict SemVer validation, allocated-version refusal, and pending-version
  precedence SHALL still inspect all local product tags.

#### Scenario: Compatibility impact selects the version increment

- **WHEN** a Change selects its release version
- **THEN** Change and release review SHALL determine the SemVer increment from
  public compatibility impact
- **AND** version syntax and chronology checks SHALL NOT infer breaking behavior.

#### Scenario: Platform trust is qualified independently of version syntax

- **WHEN** AIGW constructs or distributes a macOS or Windows artifact
- **THEN** credential-free macOS builds SHALL retain ad-hoc signatures,
  Hardened Runtime and release-epoch timestamps
- **AND** public macOS distribution SHALL require Developer ID signing and
  accepted notarization before publication
- **AND** Windows Authenticode status SHALL remain explicit
- **AND** consumers SHALL require no publisher secret or developer membership.

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

- **WHEN** a reachable product tag lacks a released section, a historical released
  section lacks its tag, or the selected tag differs from `VERSION`, the first
  released section, or `HEAD`
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
