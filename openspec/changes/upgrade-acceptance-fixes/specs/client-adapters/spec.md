# Spec Delta

## ADDED Requirements

### Requirement: Client verification runs outside a parent agent session

A client-depth check SHALL start each native client without the variables
through which a parent agent session configures its own client, so that a
check run from inside such a session verifies the client's own files and
credential command.

#### Scenario: The check runs inside a Claude Code session

- **WHEN** `aigw check --depth client` runs with `CLAUDECODE`, `CLAUDE_CODE_*`
  or `ANTHROPIC_DEFAULT_*` variables set by a parent session
- **THEN** the native client SHALL start without them
- **AND** the result SHALL match the same check run from a clean environment.
