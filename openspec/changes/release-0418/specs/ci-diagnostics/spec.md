## ADDED Requirements

### Requirement: Release selection ignores transport tags

Release tooling SHALL select a release only from a tag whose name starts with
`v`. A manual dispatch from any tag ref SHALL honor its selected native
platform.

#### Scenario: A transport tag is checked out

- **WHEN** the selected ref is a tag that does not start with `v`
- **THEN** release selection SHALL report no release tag.

#### Scenario: A maintainer dispatches one platform from a tag ref

- **WHEN** a manual Verify run from a tag ref selects one native platform
- **THEN** only that platform's native job SHALL run.
