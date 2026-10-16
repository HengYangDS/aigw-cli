# Spec Delta

## ADDED Requirements

### Requirement: Synchronization adopts a Hermes model choice

When Hermes' own model selection changed to another model in the catalogue that
AIGW projected, and nothing else that AIGW owns in the Hermes configuration
changed, `aigw sync` SHALL record the one matching Route as the Hermes Client
Binding instead of refusing. The match SHALL use the projected provider's
Account and protocol and the Route's exact upstream model. Synchronization
SHALL NOT rewrite the native selection to make it match the previous binding.

#### Scenario: The operator picks another projected model in Hermes

- **GIVEN** AIGW projected several Routes into Hermes' catalogue
- **WHEN** the operator selects another of those models in Hermes and runs
  `aigw sync`
- **THEN** the Hermes Client Binding SHALL name the matching Route
- **AND** Hermes SHALL keep the operator's selection
- **AND** the result SHALL report the adopted Route
- **AND** other clients, credentials and Hermes sessions SHALL remain unchanged.

#### Scenario: Several Routes match the native choice

- **WHEN** more than one admitted Route has the selected provider's Account,
  protocol and upstream model
- **THEN** synchronization SHALL fail without changing configuration or client
  files
- **AND** it SHALL name `aigw use --for hermes` with each matching Route.

#### Scenario: Other managed Hermes settings changed

- **WHEN** a managed provider entry, endpoint or credential command changed
  outside AIGW
- **THEN** synchronization SHALL refuse without changing configuration or client
  files.

#### Scenario: An explicit selection names another Route

- **WHEN** the operator runs `aigw use --for hermes <route>` after choosing a
  different model in Hermes
- **THEN** AIGW SHALL project the named Route, not the native choice.
