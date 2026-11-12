# Spec Delta

## ADDED Requirements

### Requirement: Client verification runs outside a parent agent session

A client-depth check SHALL start each native client without the variables
through which a parent agent session configures its own client, and with the
user's own settings for that client, so that a check run from inside such a
session verifies the client's own files and credential command as the user's
own use of the client would.

#### Scenario: The check runs inside a Claude Code session

- **WHEN** `aigw check --depth client` runs inside a parent Claude Code
  session, which sets its marker, its host and session variables and its
  model mapping
- **THEN** the native client SHALL start without them
- **AND** it SHALL start with the user's own connection, runtime and
  provider-choice settings
- **AND** the result SHALL match the same check run outside that session.
