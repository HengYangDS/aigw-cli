## ADDED Requirements

### Requirement: Client deactivation is ownership-bounded

AIGW SHALL withdraw integration through the guarded projection transaction that
created it. Disable SHALL remove only that client's owned projection and obsolete
checkpoint; uninstall SHALL first disable all enabled Adapters and remove the
program only after withdrawal succeeds. Both SHALL preserve configuration,
Tokens, other Adapters, and neighboring user state. A retained previous
configuration MAY serve only as an explicit rollback source.

#### Scenario: Disable one client

- **WHEN** an operator disables one enabled client Adapter
- **THEN** only that client's AIGW-owned projection and ownership state are
  withdrawn
- **AND** the other client, capability configuration, credentials, and
  user-authored client settings remain unchanged
- **AND** the stale verified checkpoint is absent.

#### Scenario: A user extends settings created by AIGW

- **GIVEN** AIGW created a previously absent client settings file
- **AND** the user subsequently added fields outside AIGW's ownership
- **WHEN** the Adapter is disabled or the portable program is uninstalled
- **THEN** those user fields remain and only AIGW-owned fields are withdrawn
- **AND** the file is deleted only when withdrawal leaves it empty.

#### Scenario: Uninstall the portable program

- **WHEN** an operator uninstalls AIGW with one or more enabled client Adapters
- **THEN** every AIGW-owned client projection is withdrawn before the executable
  and its program rollback copy are removed
- **AND** capability configuration, credentials, neighboring user state, and the
  explicit previous-configuration backup remain available
- **AND** no verified checkpoint continues to claim the withdrawn projections.

### Requirement: Synchronization adopts a Hermes model choice

When Hermes' own model selection changed to another model in the catalog that
AIGW projected, and nothing else that AIGW owns in the Hermes configuration
changed, `aigw sync` SHALL record the one matching Route as the Hermes Client
Binding instead of refusing. The match SHALL use the projected provider's
Account and protocol and the Route's exact upstream model. Synchronization
SHALL NOT rewrite the native selection to make it match the previous binding.

#### Scenario: The operator picks another projected model in Hermes

- **GIVEN** AIGW projected several Routes into Hermes' catalog
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

### Requirement: Synchronization adopts a Codex model choice

When Codex's own root model selection changed to the upstream model of another
Route of the selected Account and protocol, and no other value that AIGW owns
in that Codex target changed, `aigw sync` SHALL record the one matching Route
as the Codex Client Binding instead of refusing. Synchronization SHALL project
that Route without rewriting the chosen model and SHALL NOT adopt a changed
provider, provider table, scheduler, catalog or credential command.

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
  catalog changed outside AIGW, with or without a model change
- **THEN** synchronization SHALL refuse without changing configuration or client
  files.

#### Scenario: An explicit selection names another Route

- **WHEN** the operator runs `aigw use --for codex <route>` after choosing a
  different model in Codex
- **THEN** AIGW SHALL project the named Route, not the native choice.
