## ADDED Requirements

### Requirement: A model chosen in Codex or Hermes is that client's selection

When Codex's or Hermes's own selection names another added model, and nothing else that AIGW owns in that client's
file changed, every command SHALL work from that model as the client's selection. A command that saves the
configuration SHALL record it and keep the client's value. The match SHALL use the model's exact ID and the provider
and protocol of the Hermes provider entry, or those of Codex's selection.

#### Scenario: The operator picks another listed model in Hermes

- **GIVEN** AIGW wrote several models into Hermes' catalog
- **WHEN** the operator selects another of them in Hermes and runs `aigw sync`
- **THEN** Hermes's selection SHALL name the matching `provider/model`
- **AND** Hermes SHALL keep the operator's choice
- **AND** other clients, credentials and Hermes sessions SHALL remain unchanged.

#### Scenario: The operator picks another model of the same provider in Codex

- **GIVEN** AIGW wrote a model into Codex's configuration
- **WHEN** the operator selects the ID of another added model of the same provider and protocol in Codex and runs
  `aigw sync`
- **THEN** Codex's selection SHALL name that model
- **AND** Codex SHALL keep the operator's model
- **AND** a following preview SHALL report no change.

#### Scenario: A command changes the client's own settings

- **GIVEN** the operator selected another added model in Codex or Hermes
- **WHEN** a command changes another setting of that client, such as `aigw client edit` of its retry budget or its
  forwarding
- **THEN** the command SHALL succeed with its own change
- **AND** the client SHALL keep the operator's model, and the client's selection SHALL name it.

#### Scenario: A command writes none of the client's files

- **WHEN** a command saves a change that touches nothing in the files of the client whose model the operator chose
- **THEN** it SHALL record the choice as that client's selection
- **AND** it SHALL leave that client's file as it is.

#### Scenario: A client is withdrawn after a choice

- **WHEN** the operator runs `aigw client disable` for that client, or `aigw self uninstall`
- **THEN** AIGW SHALL withdraw its settings from the client's file, the selection included, as from a file that shows
  the recorded model.

#### Scenario: Other managed settings changed

- **WHEN** a managed Hermes provider entry, endpoint or credential command, the address or mode of Hermes's own
  selection, or Codex's provider selection, provider table, scheduler, catalog or credential command changed outside
  AIGW, with or without a model change
- **THEN** a command that would write that client's files SHALL refuse without changing configuration or client
  files.

### Requirement: Commands and reports name a followed selection

A command that saves the configuration, its preview, `aigw status` and `aigw check` SHALL name each model that they
follow from a client's own file, with the selection that AIGW held before and the command that selects that one
again. JSON documents SHALL carry them as `followed_selections`, with `client`, `model`, `earlier` and
`returns_with`; human output SHALL end with them. A report SHALL write nothing.

#### Scenario: A saving command names what it recorded

- **GIVEN** the operator selected another added model in Codex
- **WHEN** a command saves the configuration
- **THEN** its result SHALL name Codex, the followed model, the earlier selection and
  `aigw use codex <provider/model>` for the earlier one
- **AND** a later command SHALL name nothing, because the selection is recorded.

#### Scenario: A preview names what the run would record

- **WHEN** such a command runs with `--dry-run`
- **THEN** its preview SHALL name the same followed selection
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: Status and check after a choice

- **GIVEN** the operator selected another added model in Codex or Hermes
- **WHEN** `aigw status` or `aigw check <client> --depth local` runs
- **THEN** the client's row SHALL name that model, and the check SHALL pass for it
- **AND** the output SHALL name the followed selection
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: An explicit selection replaced the choice

- **WHEN** `aigw use <client> <provider/model>` runs for the client whose model the operator chose
- **THEN** its preview SHALL name the choice that it would replace
- **AND** its result SHALL name no followed selection for that client.

#### Scenario: The earlier model cannot be named in a shell

- **WHEN** the earlier selection's model ID would need quoting in the platform's shells
- **THEN** the JSON entry SHALL omit `returns_with`
- **AND** human output SHALL give the step in prose.

### Requirement: A client's model that AIGW has not added is reported, not followed

When a client's own selection names no added model within the client's reach, and nothing else that AIGW owns
changed, a command that would write that client's files SHALL fail without changing configuration or client files;
any other command SHALL leave that client as recorded. The Fix SHALL restore the recorded selection with
`aigw use <client> <provider/model>`, and the failure SHALL name the command that adds and selects the named model
when AIGW wrote that model's provider.

#### Scenario: Codex names a model that its provider has not added

- **WHEN** Codex's root model names an ID that is not an added model of Codex's provider and protocol, and
  `aigw sync` runs
- **THEN** synchronization SHALL fail without changing configuration or client files
- **AND** its Fix SHALL be `aigw use codex <provider/model>` for the recorded selection
- **AND** it SHALL name `aigw use codex <provider>/<that ID>` as the command that adds the model
- **AND** the Fix, run as printed, SHALL write the recorded model, after which `aigw sync` SHALL succeed.

#### Scenario: Hermes names a model that its provider entry does not list

- **WHEN** Hermes's selection names a model that the selected provider entry does not list, and `aigw sync` runs
- **THEN** synchronization SHALL fail without changing configuration or client files
- **AND** its Fix SHALL be `aigw use hermes <provider/model>` for the recorded selection, with its protocol
- **AND** the Fix, run as printed, SHALL write the recorded model, after which `aigw sync` SHALL succeed.

#### Scenario: Another command and a withdrawal meet the same file

- **WHEN** a command that changes that client's settings, or `aigw client disable` for it, runs while its selection
  names such a model
- **THEN** it SHALL fail with the same Fix and change nothing.

#### Scenario: Status names the same Fix

- **WHEN** `aigw status` or `aigw check` runs while a client's selection names such a model
- **THEN** the client's row SHALL name the model that the client's file selects and the recorded selection
- **AND** its next action SHALL be the same Fix.

#### Scenario: The recorded model cannot be named in a shell

- **WHEN** the recorded selection's model ID would need quoting in the platform's shells
- **THEN** the Fix SHALL be prose in `next_detail`, and `next_action` SHALL be absent.

### Requirement: An explicit selection replaces the client's own

`aigw use <client> <provider/model>` for Codex or Hermes SHALL write the named model whatever model the client's own
selection names, a model that AIGW has not added included, when nothing else that AIGW owns in the client's file
changed.

#### Scenario: An explicit selection names another model

- **WHEN** the operator runs `aigw use <client> <provider/model>` for Hermes or Codex after choosing a different
  model in that client
- **THEN** AIGW SHALL write the named model, not the client's choice.

#### Scenario: An explicit selection over a model that AIGW has not added

- **WHEN** the client's selection names a model that AIGW has not added and the operator runs
  `aigw use <client> <provider/model>`
- **THEN** AIGW SHALL write the named model and the client's files SHALL match the selection.

#### Scenario: Hermes's selection has an edited address

- **WHEN** the address or mode of Hermes's own selection differs from its provider entry and the operator runs
  `aigw use hermes <provider/model>`
- **THEN** AIGW SHALL refuse as for any other managed setting changed outside AIGW.

### Requirement: Claude Code's own model is reported as it stands

When Claude Code's settings name a model of their own while the connection that AIGW wrote is intact, `aigw status`
and `aigw check` SHALL name that value beside AIGW's selection, in human output and as `client_model` in JSON, and
no command SHALL change it except an explicit `aigw use claude`.

#### Scenario: Status names Claude Code's own model

- **GIVEN** Claude Code's settings name `opus[1m]` and AIGW selected another model for it
- **WHEN** `aigw status` runs
- **THEN** Claude Code's row SHALL name `opus[1m]` as its own model preference, and AIGW's selection
- **AND** the JSON document SHALL carry both values.
