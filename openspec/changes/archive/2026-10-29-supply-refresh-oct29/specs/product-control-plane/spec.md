## ADDED Requirements

### Requirement: A vulnerable exact transitive pin is overridden

When a direct dependency pins a transitive release that a published advisory
places in its vulnerable range, the package manifest SHALL override that pin,
scoped to that dependency, with the highest fixed stable release. The override
SHALL leave when a release of the dependency requires a fixed release itself.

#### Scenario: A direct dependency pins a vulnerable release exactly

- **WHEN** an advisory places a release that a direct dependency pins exactly
  in its vulnerable range, such as re2js 2.8.6 in markdownlint 0.42.0
- **THEN** the manifest SHALL override that dependency's pin with the highest
  fixed stable release, without the ordinary release-age delay
- **AND** the lock SHALL change only for that override, and the repository's
  checks SHALL pass with it before integration.

#### Scenario: The dependency adopts a fixed release

- **WHEN** the refresh adopts a release of the dependency that requires a fixed
  release itself
- **THEN** the same refresh SHALL remove the override
- **AND** the lock SHALL resolve the dependency's own requirement.
