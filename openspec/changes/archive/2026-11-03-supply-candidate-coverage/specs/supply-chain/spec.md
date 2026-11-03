## ADDED Requirements

### Requirement: Candidate calculation reads every pin

The candidate calculation SHALL read every version and commit pin of the
repository's dependency authorities from the datasource of its publisher. The
supply chain SHALL NOT be described as current while a pin is unread.

#### Scenario: A pin moves to a different carrier

- **WHEN** a pin of the CUE authority, such as a native client version or the
  Hermes release commit, moves to a different carrier
- **THEN** the candidate calculation SHALL still read it with its pinned value
- **AND** the repository's dependency checks SHALL fail while a native client
  pin is read by no candidate rule or by more than one.

#### Scenario: A pinned client publishes a newer stable release

- **WHEN** a native client or Hermes publishes a newer stable release
- **THEN** the candidate calculation SHALL report it as an update, or as held
  by the minimum release age with the time when that age ends.
