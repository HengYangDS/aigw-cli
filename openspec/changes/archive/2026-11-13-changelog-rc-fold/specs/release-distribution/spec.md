## MODIFIED Requirements

### Requirement: Version and changelog identity are consistent

`VERSION` SHALL be the sole strict-SemVer authority. `CHANGELOG.md` SHALL use
Keep a Changelog 1.1.0 with one leading `Unreleased`, canonical categories and
descending released versions, each with its local product tag except one newer
untagged pending release. Tag, `VERSION`, first released section and `HEAD` SHALL
agree before publication. Change and release review, not syntax or chronology,
SHALL choose the SemVer increment from public compatibility impact.

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

- **WHEN** a historical released section lacks its tag, or the selected tag
  differs from `VERSION`, the first released section, or `HEAD`
- **THEN** the release gate SHALL fail before construction or publication.

#### Scenario: Release metadata exists without publication

- **WHEN** `VERSION` and `CHANGELOG` name a release but either selected peer
  lacks its exact signed tag object, Release record, or assets
- **THEN** delivery SHALL remain incomplete.

#### Scenario: Pre-release candidates precede their release

- **WHEN** a release follows pre-release candidates, such as 0.1.0 after
  `0.1.0-rc.58` to `0.1.0-rc.118`
- **THEN** the release's section SHALL carry the candidates' changes under its
  categories, and no candidate SHALL keep a section of its own
- **AND** a product tag without a section SHALL NOT fail the release gate.
