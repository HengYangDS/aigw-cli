## ADDED Requirements

### Requirement: Claude Code maps Anthropic model IDs to the provider's

AIGW SHALL write into Claude Code's settings one map from each Anthropic model ID that a Claude model of the selected
provider serves to one ID of that provider. A model serves the Anthropic model ID that is its family, when that
family begins with `claude-` and is not the model's own ID. The selected model's ID SHALL win its family; another
family SHALL take its first such model in configuration order. The map SHALL be a value that AIGW owns, records and
withdraws.

#### Scenario: Claude Code's picker rows reach the provider's channel

- **GIVEN** the selected provider serves Claude Fable 5.2 and Claude Opus 5.5 only as `claude-code-fable-5-2` and
  `claude-code-opus-5-5`, each in the family of the Anthropic model ID
- **WHEN** AIGW projects Claude Code's settings
- **THEN** the settings SHALL map `claude-fable-5-2` to `claude-code-fable-5-2` and `claude-opus-5-5` to
  `claude-code-opus-5-5`
- **AND** a model whose family is not an Anthropic model ID, or is its own ID, SHALL be absent from the map
- **AND** a map that would be empty SHALL not be written.

#### Scenario: The selected channel wins its family

- **WHEN** the selected model and another model of the selected provider share the family `claude-opus-5-5`
- **THEN** the map SHALL send `claude-opus-5-5` to the selected model's ID.

#### Scenario: The map is withdrawn with Claude Code's projection

- **WHEN** AIGW withdraws Claude Code's projection
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, absent when they held none.

#### Scenario: The map changed outside AIGW

- **WHEN** the map in Claude Code's settings differs from what AIGW recorded and the next transition writes them
- **THEN** the transition SHALL refuse it as managed drift and write nothing.

### Requirement: Claude Desktop names the Anthropic model behind an entry

AIGW SHALL name, on each model entry of its Claude Desktop profile, the Anthropic model ID that the entry's model
serves, as the Claude Code map defines it, and no model on an entry that serves none. The name SHALL be part of the
profile that AIGW owns.

#### Scenario: Claude Desktop names the model behind each entry

- **WHEN** AIGW projects the Claude Desktop profile with `claude-code-fable-5-2` in the family `claude-fable-5-2`
- **THEN** that entry SHALL name `claude-fable-5-2` as the model behind it
- **AND** an entry whose ID is its family SHALL name none.
