## ADDED Requirements

### Requirement: Client readiness is evidence-bounded

AIGW SHALL report projection readiness and native client authentication as
separate facts and SHALL NOT describe Codex as fully ready unless the public
Codex status command proves authentication for the selected target.

#### Scenario: Projection exists without native authentication

- **WHEN** the Codex projection, Token, and endpoint are available
- **AND** the public Codex login-status command does not prove authentication
- **THEN** status reports the projection as ready
- **AND** reports native authentication as required
- **AND** names aigw adapter auth codex as the next action

#### Scenario: Native authentication is proved

- **WHEN** the public Codex login-status command succeeds for every selected target
- **THEN** status may report Codex as locally ready
- **AND** the JSON projection records native authentication as present
