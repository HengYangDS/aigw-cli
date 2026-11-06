## ADDED Requirements

### Requirement: Tracked content names only the public peer

Tracked content SHALL name no publication peer other than the public GitHub
repository: no private peer's host, port, namespace, project path or project
ID. A private peer SHALL be resolved from the clone's local Git configuration.

#### Scenario: A release names its comparison links

- **WHEN** CHANGELOG lists the comparison links of a release
- **THEN** it SHALL link only to the public GitHub repository.

#### Scenario: A test needs a private forge

- **WHEN** a test exercises a private forge
- **THEN** it SHALL use a fixture host and project ID
- **AND** it SHALL NOT contain the operator's private peer identity.

#### Scenario: ETHOS publishes to the private peer

- **WHEN** a branch or tag is published to the private GitLab peer
- **THEN** its repository SHALL come from the clone's
  `remote.gitlab.forgeRepository`
- **AND** tracked configuration SHALL name only the peer's Git remote.
