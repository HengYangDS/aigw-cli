## ADDED Requirements

### Requirement: Hermes selection says existing sessions keep their route

When `aigw use` changes, or previews a change to, the projection of a client
whose existing sessions keep the route they were created with, such as Hermes,
human output SHALL say that existing sessions keep that route and new sessions
use the selection. AIGW SHALL NOT change a native session's route.

#### Scenario: An operator selects another Route for Hermes

- **WHEN** `aigw use --for hermes` changes the Hermes projection, or its preview
  would
- **THEN** human output SHALL state that existing sessions keep their route and
  new sessions use the selection
- **AND** JSON output and Hermes sessions SHALL stay unchanged.

#### Scenario: The projection stays the same

- **WHEN** a selection leaves the Hermes projection unchanged
- **THEN** human output SHALL NOT add the statement.
