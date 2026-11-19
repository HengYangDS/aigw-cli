## ADDED Requirements

### Requirement: A model chosen in Codex or Hermes is that client's selection

When Codex's or Hermes's own selection names another added model, and nothing else that AIGW owns in that client's
file changed, every command that writes or previews the client's files SHALL record that model as the client's
selection, keep the client's value, and name the followed model with the command that returns to the earlier one. The
match SHALL use the model's exact ID and the provider and protocol of the Hermes provider entry, or those of Codex's
selection.

#### Scenario: The operator picks another listed model in Hermes

- **GIVEN** AIGW wrote several models into Hermes' catalog
- **WHEN** the operator selects another of them in Hermes and runs `aigw sync`
- **THEN** Hermes's selection SHALL name the matching `provider/model`
- **AND** Hermes SHALL keep the operator's choice
- **AND** the result SHALL report the followed model
- **AND** other clients, credentials and Hermes sessions SHALL remain unchanged.

#### Scenario: The operator picks another model of the same provider in Codex

- **GIVEN** AIGW wrote a model into Codex's configuration
- **WHEN** the operator selects the ID of another added model of the same provider and protocol in Codex and runs
  `aigw sync`
- **THEN** Codex's selection SHALL name that model
- **AND** Codex SHALL keep the operator's model
- **AND** the result SHALL report the followed model
- **AND** a following preview SHALL report no change.

#### Scenario: Another command writes the client's files

- **GIVEN** the operator selected another added model in Codex or Hermes
- **WHEN** a command other than `aigw sync` changes that client's files, such as `aigw provider edit` of the
  selected provider's label
- **THEN** the command SHALL succeed, the client SHALL keep the operator's model, and the client's selection SHALL
  name it
- **AND** the result SHALL name the followed model and `aigw use <client> <provider/model>` for the earlier
  selection.

#### Scenario: A preview names the followed model

- **WHEN** such a command runs with `--dry-run`
- **THEN** its preview SHALL name the followed model
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: A command writes none of the client's files

- **WHEN** a command changes nothing in the files of the client whose model the operator chose
- **THEN** it SHALL succeed and SHALL leave that client's file and recorded selection as they are.

#### Scenario: Other managed settings changed

- **WHEN** a managed Hermes provider entry, endpoint or credential command, or Codex's provider selection, provider
  table, scheduler, catalog or credential command changed outside AIGW, with or without a model change
- **THEN** a command that would write that client's files SHALL refuse without changing configuration or client
  files.

### Requirement: Status and check report a model chosen in the client

`aigw status` and `aigw check` SHALL report a model that a client's own selection names, and that a writing command
would follow, as the client's selection, marked as chosen in the client, in human and JSON output. They SHALL judge
the client's files against that model and SHALL write nothing.

#### Scenario: Status after a choice in Codex

- **GIVEN** the operator selected another added model in Codex
- **WHEN** `aigw status` runs
- **THEN** Codex's row SHALL name that model as configured and say that it was chosen in Codex
- **AND** the JSON document SHALL name that model and mark the selection as chosen in the client
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: Check after a choice in Hermes

- **GIVEN** the operator selected another listed model in Hermes
- **WHEN** `aigw check hermes --depth local` runs
- **THEN** it SHALL pass for that model and say that it was chosen in Hermes.

### Requirement: A client's model that AIGW has not added is reported, not followed

When a client's own selection names a model that is not an added model within the client's reach, and nothing else
that AIGW owns changed, a command that would write that client's files SHALL fail without changing configuration or
client files. Its Fix SHALL be `aigw use <client> <provider/model>` for the recorded selection, and it SHALL name the
command that adds and selects the named model when AIGW wrote that model's provider. `aigw status` and `aigw check`
SHALL report the same Fix.

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

#### Scenario: Status names the same Fix

- **WHEN** `aigw status` or `aigw check` runs while a client's selection names such a model
- **THEN** the client's row SHALL name the model that the client's file selects and the recorded selection
- **AND** its next action SHALL be the same Fix.

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

### Requirement: Claude Code's own model is reported as it stands

When Claude Code's settings name a model of their own while the connection that AIGW wrote is intact, `aigw status`
and `aigw check` SHALL name that value as the model that Claude Code uses, beside AIGW's selection, in human and JSON
output, and no command SHALL change it except an explicit `aigw use claude`.

#### Scenario: Status names Claude Code's own model

- **GIVEN** Claude Code's settings name `opus[1m]` and AIGW selected another model for it
- **WHEN** `aigw status` runs
- **THEN** Claude Code's row SHALL name `opus[1m]` as set in Claude Code, and AIGW's selection
- **AND** the JSON document SHALL carry both values.
