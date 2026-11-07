## ADDED Requirements

### Requirement: Tracked content holds no peer coordinates

The changelog and the publication declaration SHALL name no publication peer's
repository, and no tracked content SHALL name the private peer's host, port,
namespace, project path or project ID. Each peer SHALL be resolved from the
clone's Git configuration.

#### Scenario: A release is recorded in the changelog

- **WHEN** CHANGELOG records a release
- **THEN** it SHALL keep the release heading and its text
- **AND** it SHALL contain no comparison link or other peer URL.

#### Scenario: A test needs a private forge

- **WHEN** a test exercises a private forge
- **THEN** it SHALL use a fixture host and project ID
- **AND** it SHALL NOT contain the operator's private peer identity.

#### Scenario: ETHOS publishes to a peer

- **WHEN** a branch or tag is published to a peer
- **THEN** its repository SHALL come from the clone's Git configuration
- **AND** tracked configuration SHALL name only the peer's Git remote.
