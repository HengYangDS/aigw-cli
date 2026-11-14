## ADDED Requirements

### Requirement: Retained job evidence has one declared lifetime

Every artifact that a hosted job retains SHALL declare its expiry as one number
of days, which the CI model declares once and both Forge projections carry.

#### Scenario: A job retains evidence

- **WHEN** a GitHub job uploads an artifact or a GitLab job declares artifacts
- **THEN** the upload step SHALL carry the declared number as its retention,
  and the artifact declaration SHALL carry it as its expiry
- **AND** the projection test SHALL fail for an upload step or an artifact
  declaration without it, or with another number.

#### Scenario: The lifetime changes

- **WHEN** the declared number of days changes
- **THEN** one field of the CI model changes, and regeneration carries it into
  both projections.
