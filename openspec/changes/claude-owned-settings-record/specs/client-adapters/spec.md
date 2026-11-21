## MODIFIED Requirements

### Requirement: Claude Code maps Anthropic model IDs to the provider's

AIGW SHALL write into Claude Code's settings one map from each Anthropic model ID that a listed model serves to that
model's ID. A model serves the Anthropic model ID that is its family, when that family begins with `claude-` and is
not the model's own ID. The listed models are the selected model and the first model of each other family of its
provider on the selected protocol, in configuration order. The map SHALL be a value that AIGW owns, records and
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

#### Scenario: A family's first model names its family

- **WHEN** the provider lists `claude-fable-5-2` before `claude-code-fable-5-2`, both in the family
  `claude-fable-5-2`, and neither is selected
- **THEN** the map SHALL hold no entry for `claude-fable-5-2`, since the listed model is the Anthropic ID itself.

#### Scenario: Only the map changes

- **WHEN** a command changes which models the provider lists, and nothing else of Claude Code's selection
- **THEN** that command SHALL rewrite Claude Code's settings with the new map.

#### Scenario: The map is withdrawn with Claude Code's projection

- **WHEN** AIGW withdraws Claude Code's projection
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, absent when they held none.

#### Scenario: The map changed outside AIGW

- **WHEN** the map in Claude Code's settings differs from what AIGW recorded and the next transition writes them
- **THEN** the transition SHALL refuse it as managed drift and write nothing.

#### Scenario: The settings hold the user's own map

- **GIVEN** Claude Code's settings hold a map that AIGW did not write
- **WHEN** AIGW projects a selection that needs a map
- **THEN** it SHALL write its map and keep the user's for the withdrawal
- **AND** while the selection needs no map, AIGW SHALL leave the user's map as it is.

#### Scenario: The selection no longer needs a map

- **WHEN** AIGW projects a selection that needs no map over settings that hold the map that AIGW wrote
- **THEN** the map SHALL return to what the settings held before AIGW wrote it.

## ADDED Requirements

### Requirement: A record that the previous release reads keeps its shape

A record of a client's file that the previous published release reads strictly SHALL keep the fields and the digest
that that release reads. AIGW SHALL record a setting that it newly owns in that file in a record of its own beside
the first, whose reader keeps an entry that it does not know and leaves that entry's setting as it is. Release
acceptance SHALL roll the program back to the published predecessor with such a setting written.

#### Scenario: The previous release synchronizes after a rollback

- **GIVEN** the newer program wrote Claude Code's settings with a map
- **WHEN** the program is rolled back to the previous release and that release synchronizes
- **THEN** the synchronization SHALL succeed with Claude Code enabled
- **AND** the map SHALL stay in the settings as the newer program wrote it.

#### Scenario: The newer program returns after a rollback

- **WHEN** the newer program is activated again after the previous release synchronized, and synchronizes
- **THEN** it SHALL find the map its own, however the previous release re-indented the settings, and write the
  selection's map.

#### Scenario: The previous release withdrew the client

- **GIVEN** the previous release withdrew Claude Code after a rollback, which left the map and its record
- **WHEN** the newer program next projects or withdraws Claude Code, or is uninstalled
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, and its record SHALL be removed.

#### Scenario: The record holds an entry of a later release

- **WHEN** the record of newly owned settings holds an entry that this program does not know
- **THEN** this program SHALL keep the entry when it writes the record and SHALL leave that entry's setting as it
  is.
