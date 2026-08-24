## ADDED Requirements

### Requirement: Deferred client activation remains explicit

AIGW SHALL allow team configuration import before clients or Tokens exist and
SHALL guide later client installation through projection, explicit native
authentication, and verification without making every provider mandatory.

#### Scenario: Codex is installed after setup

- **WHEN** an operator installs Codex after importing team configuration
- **THEN** aigw sync creates only the AIGW-owned projection
- **AND** status identifies explicit authentication as the next step when needed
- **AND** no unrelated provider Token is required
