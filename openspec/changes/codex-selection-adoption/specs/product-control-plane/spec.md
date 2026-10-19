## ADDED Requirements

### Requirement: Synchronization adopts a Codex model choice

When Codex's own root model selection changed to the upstream model of another
Route of the selected Account and protocol, and no other value that AIGW owns
in that Codex target changed, `aigw sync` SHALL record the one matching Route
as the Codex Client Binding instead of refusing. Synchronization SHALL project
that Route without rewriting the chosen model and SHALL NOT adopt a changed
provider, provider table, scheduler, catalogue or credential command.

#### Scenario: The operator picks another model of the same Account in Codex

- **GIVEN** AIGW projected a Route into Codex
- **WHEN** the operator selects the upstream model of another Route of the same
  Account and protocol in Codex and runs `aigw sync`
- **THEN** the Codex Client Binding SHALL name the matching Route
- **AND** Codex SHALL keep the operator's model
- **AND** the result SHALL report the adopted Route
- **AND** a following dry run SHALL report no change.

#### Scenario: No Route or several Routes match the native choice

- **WHEN** no Route, or more than one Route, of the selected Account and
  protocol has the chosen upstream model
- **THEN** synchronization SHALL fail without changing configuration or client
  files
- **AND** it SHALL name `aigw use --for codex` with each matching Route, or the
  selection command when none matches.

#### Scenario: Other managed Codex settings changed

- **WHEN** the provider selection, the provider table, the scheduler or the
  catalogue changed outside AIGW, with or without a model change
- **THEN** synchronization SHALL refuse without changing configuration or client
  files.

#### Scenario: An explicit selection names another Route

- **WHEN** the operator runs `aigw use --for codex <route>` after choosing a
  different model in Codex
- **THEN** AIGW SHALL project the named Route, not the native choice.
