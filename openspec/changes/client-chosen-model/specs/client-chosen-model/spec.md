## ADDED Requirements

### Requirement: A model chosen in Codex or Hermes is that client's selection

When Codex's or Hermes's own selection names another added model, and nothing else that AIGW owns in that client's
file changed, every command SHALL work from that model as the client's selection, matched by its exact ID with the
provider and protocol of Hermes's provider entry or of Codex's selection. A command that saves the configuration
SHALL record it only together with AIGW's record of the client's file; under every other requirement that is no
change of selection.

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

#### Scenario: A command about something else records the choice

- **WHEN** a command saves a change that names nothing of the client whose model the operator chose
- **THEN** it SHALL record the choice as that client's selection
- **AND** it SHALL bring AIGW's record of that client's file to the chosen model, which the file keeps, and name
  the client among those whose files it wrote
- **AND** the earlier model, selected in the client again afterwards, SHALL be followed as any other choice.

#### Scenario: Codex has several files

- **WHEN** one or more of Codex's files name the same other added model and the others show the recorded one
- **THEN** that model SHALL be Codex's selection, `aigw status` and `aigw check` SHALL report Codex as configured,
  and a command that records it SHALL write it to every file
- **AND** files that name different models SHALL be followed by no command; a command that writes them SHALL fail
  with the step that restores the recorded selection.

#### Scenario: The chosen model's provider waits for a Token

- **WHEN** the provider of the chosen model has no Token that AIGW can use and a command saves the configuration
- **THEN** it SHALL leave the client's file and the stored selection as they are, so that the choice stays in the
  client's file alone
- **AND** `aigw status` SHALL name the chosen model for that client, as waiting for the Token
- **AND** the first command that saves after the Token is stored SHALL record the model that the file selects then.

#### Scenario: A command changes the settings of a client that waits for a Token

- **GIVEN** the operator selected in Hermes a model whose provider has no Token that AIGW can use
- **WHEN** a command changes another setting of that client, such as `aigw client edit hermes --direct` or `--via`
- **THEN** the command SHALL save its change on the stored selection and name that selection in its result
- **AND** it SHALL leave the client's file as it is
- **AND** the first command that saves after the Token is stored SHALL record the choice and name a forwarding that
  the choice's upstream ends.

#### Scenario: Another file of the client cannot be written

- **WHEN** a command that names nothing of the client saves while another of that client's files is missing or
  refused
- **THEN** the command SHALL succeed as it does without the choice
- **AND** it SHALL leave the choice in the client's file alone.

#### Scenario: Any command after a choice

- **WHEN** a command that neither selects a model for that client nor rolls the configuration back runs after such
  a choice
- **THEN** it SHALL NOT fail or refuse because of the choice, at any depth of `aigw check`
- **AND** a check of every client that passes SHALL write its verified snapshot.

#### Scenario: A client is withdrawn after a choice

- **WHEN** the operator runs `aigw client disable` for that client, or `aigw self uninstall`
- **THEN** AIGW SHALL withdraw its settings from the client's file, the selection included, as from a file that shows
  the recorded model.

#### Scenario: Other managed settings changed

- **WHEN** a managed Hermes provider entry, endpoint or credential command, the address or mode of Hermes's own
  selection, or Codex's provider selection, provider table, scheduler, catalog or credential command changed outside
  AIGW, with or without a model change
- **THEN** no command SHALL follow that client's model
- **AND** a command that would write that client's files SHALL refuse without changing configuration or client
  files.

### Requirement: Commands and reports name a followed selection

A command that records a followed selection, its preview, `aigw status`, `aigw check` and bare `aigw` SHALL name each
model that they follow from a client's own file, the selection that AIGW held before, and the command that selects
that one again. JSON documents SHALL carry them as `followed_selections`; human output SHALL name them in a section
ahead of the next step. A report SHALL write nothing, and any other command SHALL name none.

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
- **AND** the output SHALL name the followed selection, also when the check fails for another reason
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: The fields of an entry

- **WHEN** a JSON document names a followed selection
- **THEN** its entry SHALL carry `client`, `model` and `earlier`, and for a client that speaks several protocols
  `protocol` and `earlier_protocol`
- **AND** it SHALL carry `returns_with`, the command that selects the earlier model again, while the configuration
  holds that model and a shell passes its ID unquoted
- **AND** `forwarding_cleared` or `retries_cleared` SHALL be true when the client's forwarding or its retry budgets
  ended with the earlier upstream endpoint or provider, and human output SHALL say so.

#### Scenario: A command that fails names none

- **WHEN** a command that would have recorded the choice fails
- **THEN** its output SHALL name no followed selection, in text or JSON
- **AND** a report that fails, such as a check that stops at a depth, SHALL still name what it follows.

#### Scenario: An explicit selection replaced the choice

- **WHEN** `aigw use <client> <provider/model>` runs for the client whose model the operator chose
- **THEN** neither its preview nor its result SHALL name a followed selection for that client
- **AND** when it names the chosen model itself, it SHALL record that model and report, in preview and result, that
  the configuration changes.

#### Scenario: The command removes the earlier model

- **WHEN** the command that records the choice removes the earlier selection's model
- **THEN** the entry SHALL omit `returns_with`
- **AND** human output SHALL say that the configuration no longer holds that model.

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

#### Scenario: A preview is refused in the same words

- **WHEN** the preview of such a command meets the same file
- **THEN** it SHALL fail with the problem and the Fix that the command itself gives
- **AND** both SHALL say that configuration and client files are unchanged.

#### Scenario: Status names the same Fix

- **WHEN** `aigw status` or `aigw check` runs while a client's selection names such a model
- **THEN** the client's row SHALL name the model that the client's file selects and the recorded selection
- **AND** its next action SHALL be the same Fix.

#### Scenario: Another Codex file holds an outside edit

- **WHEN** one Codex file names such a model and another, before or after it, holds an outside edit of the settings
  that AIGW manages
- **THEN** every surface SHALL give the recovery of the outside edit and SHALL NOT name the restoring command, which
  that edit would refuse
- **AND** once the edit is undone, the restoring command SHALL be the Fix and SHALL run as printed.

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

### Requirement: A rollback names the selections that it restores

`aigw config rollback` SHALL restore each client's selection as the restored configuration holds it, a model chosen
in the client since then included where it writes that client's file, and SHALL name each one with the model that it
replaces: `selections` in JSON, with `client`, `model` and `replaces`, and a section in human output, in its preview
and in its result, and no followed selection for that client. A rollback that is refused before it writes SHALL pass
on the refusal's Fix.

#### Scenario: A rollback over a model chosen in Codex

- **GIVEN** the operator chose another added model in Codex after the configuration that a rollback restores
- **WHEN** `aigw config rollback` runs, or its preview
- **THEN** it SHALL name Codex, the selection that it restores and the chosen model that this replaces
- **AND** the run SHALL write the restored selection to Codex's file, and the preview SHALL write nothing.

#### Scenario: A rollback does not write a client that waits for its Token

- **WHEN** a client whose file selects a model of its own waits for a Token and a rollback runs
- **THEN** the model chosen in the client SHALL stay in its file
- **AND** the rollback SHALL name for that client only a stored selection that it changes.

#### Scenario: A rollback restores a client without a selection

- **WHEN** the restored configuration selects no model for a client that has one now
- **THEN** the entry SHALL carry `client` and `replaces` and omit `model`.

#### Scenario: A rollback meets a model that AIGW has not added

- **WHEN** a rollback would write a client's file whose selection names a model that AIGW has not added
- **THEN** the rollback and its preview SHALL fail with the Fix that restores the recorded selection
- **AND** configuration and client files SHALL remain unchanged.

### Requirement: Claude Code's own model is reported as it stands

When Claude Code's settings name a model of their own while the connection that AIGW wrote is intact, `aigw status`
and `aigw check` SHALL name that value beside AIGW's selection, in human output and as `client_model` in JSON, and
no command SHALL change it except an explicit `aigw use claude`.

#### Scenario: Status names Claude Code's own model

- **GIVEN** Claude Code's settings name `opus[1m]` and AIGW selected another model for it
- **WHEN** `aigw status` runs
- **THEN** Claude Code's row SHALL name `opus[1m]` as its own model preference, and AIGW's selection
- **AND** the JSON document SHALL carry both values.
