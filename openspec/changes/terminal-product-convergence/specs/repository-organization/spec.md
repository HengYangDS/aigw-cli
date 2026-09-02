## ADDED Requirements

### Requirement: Logical and physical ownership are isomorphic

Source, tests, repository tools, configuration, documentation, schemas, and
generated projections SHALL be organized by stable product or repository
responsibility. Names SHALL be precise, readable words; flat suffix families,
concatenated compounds, ambiguous buckets, and implementation-shaped groupings
SHALL be replaced by cohesive semantic packages when they conceal ownership.

#### Scenario: Several files implement one responsibility

- **WHEN** a file family shares one lifecycle, dependency direction, and reason
  to change
- **THEN** it resides under one named semantic owner
- **AND** callers do not import sibling implementation details.

#### Scenario: A carrier has no current consumer

- **WHEN** a file, directory, compatibility path, evidence shell, generated
  projection, runtime, branch, tag, or configuration entry protects no current
  invariant and has no current consumer
- **THEN** it is deleted rather than retained for history
- **AND** no compatibility alias or forwarding wrapper is added.

### Requirement: Each Work Lane reconstructs its own mutable environment

Every Work Lane SHALL reconstruct its own mutable dependency and build state
from committed locks through one repository entrypoint. Work Lanes MAY share
only immutable or content-addressed caches and MUST NOT share another lane's
virtual environment, dependency tree, build output, or temporary state.

#### Scenario: A fresh Work Lane is created

- **WHEN** a contributor enters the Work Lane without `.venv`, `node_modules`,
  build output, or test state
- **THEN** one documented bootstrap command reconstructs the complete declared
  development environment
- **AND** no ambient interpreter, global configuration, or sibling checkout is
  required.

#### Scenario: A Change updates the toolchain

- **WHEN** one Work Lane changes a lock or tool version
- **THEN** only that Work Lane's mutable environment is rebuilt
- **AND** shared content-addressed caches remain safe for other lanes.

### Requirement: Repository navigation follows reader intent

Every public document and governed directory SHALL have one intentional entry
path, meaningful links to adjacent concepts and operations, and no empty index
whose only purpose is structural symmetry.

#### Scenario: A reader follows a named concept

- **WHEN** documentation refers to another internal document, command, decision,
  or external standard
- **THEN** it uses a valid navigable link when a stable target exists
- **AND** the link text states the destination's meaning rather than its file
  name alone.
