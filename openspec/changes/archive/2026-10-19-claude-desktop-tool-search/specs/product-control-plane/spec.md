## ADDED Requirements

### Requirement: Claude Desktop tool search follows Route qualification

AIGW SHALL enable Claude Desktop host tool search in its owned gateway profile
only when every Route the profile exposes declares the `tool_search`
capability on its Anthropic interface. That capability records a native
Desktop session's qualification of the tool-search beta, deferred tool loading
and `tool_reference` results on that interface. Configuration validation SHALL
reject `tool_search` on any other interface.

#### Scenario: Every exposed Route qualified tool search

- **WHEN** the selected Route and every other Route in the Desktop profile
  declare `tool_search` on their Anthropic interface
- **THEN** synchronization SHALL write the profile with tool search enabled
- **AND** inspection SHALL report that projection unchanged.

#### Scenario: One exposed Route lacks the qualification

- **WHEN** any Route in the Desktop profile does not declare `tool_search`
- **THEN** the profile SHALL omit the tool-search setting
- **AND** a tool-search setting added outside AIGW SHALL fail closed as
  managed drift.

#### Scenario: An exposed Route's qualification changes

- **WHEN** an exposed Route gains or loses `tool_search`
- **THEN** synchronization SHALL report a projection change and rewrite the
  profile to match.

#### Scenario: A non-Anthropic interface declares tool search

- **WHEN** a Route declares `tool_search` on an OpenAI interface
- **THEN** configuration validation SHALL reject that Route.
