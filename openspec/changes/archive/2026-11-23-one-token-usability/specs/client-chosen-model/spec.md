## MODIFIED Requirements

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

#### Scenario: The pick is written in another spelling of the file's format

- **GIVEN** the operator writes the model in Codex's file as a TOML literal string, without spaces around the sign
  or with a comment after it, or in Hermes's file with the model or the provider as a quoted YAML scalar
- **WHEN** a command or a report reads the file
- **THEN** the choice SHALL be read, followed and recorded as the same pick in the spelling that AIGW writes
- **AND** files of one client that name the same model in different spellings SHALL count as selecting one model.

#### Scenario: A command changes the client's own settings

- **GIVEN** the operator selected another added model in Codex or Hermes
- **WHEN** a command changes another setting of that client, such as `aigw client edit` of its retry budget or its
  forwarding
- **THEN** the command SHALL succeed with its own change
- **AND** the client SHALL keep the operator's model, and the client's selection SHALL name it
- **AND** the edit SHALL be judged against the stored selection: `aigw client edit --direct` after a choice that
  ended the stored forwarding SHALL save the choice without that forwarding, report `changed` and name the followed
  selection, so that the stored model, picked again, does not forward again.

#### Scenario: An edit asks for what is stored already

- **GIVEN** no file of the client chose a model
- **WHEN** `aigw client edit` asks for settings that the stored selection of the client already has
- **THEN** it SHALL report that nothing changed
- **AND** it SHALL write neither the configuration, a client file nor AIGW's record of one.

#### Scenario: A command about something else records the choice

- **WHEN** a command saves a change that names nothing of the client whose model the operator chose
- **THEN** it SHALL record the choice as that client's selection
- **AND** it SHALL bring AIGW's record of that client's file to the chosen model, which the file keeps, and name
  the client among those whose files it wrote
- **AND** the earlier model, selected in the client again afterwards, SHALL be followed as any other choice.

#### Scenario: The chosen model's provider waits for a Token

- **WHEN** the provider of the chosen model has no Token that AIGW can use and a command saves the configuration
- **THEN** it SHALL record the choice as that client's selection, and bring AIGW's record of the client's file to
  it without writing the file, the catalog or a credential reader
- **AND** `aigw status` and every list and view SHALL name the chosen model for that client, and the client as
  waiting for the Token of its selection's provider
- **AND** the synchronization after the Token is stored SHALL write the file, with the selection that the record
  names.

#### Scenario: Codex has several files

- **WHEN** one or more of Codex's files name the same other added model and the others show the recorded one
- **THEN** that model SHALL be Codex's selection, `aigw status` and `aigw check` SHALL report Codex as configured,
  and a command that records it SHALL write it to every file
- **AND** files that name different models SHALL be followed by no command; a command that writes them SHALL fail
  with the step that restores the recorded selection.

#### Scenario: Every list and view names the model that the client uses

- **GIVEN** a client whose file selects a model whose provider has no Token that AIGW can use
- **WHEN** `aigw client list`, `aigw model list`, `aigw model show`, `aigw provider list` or `aigw provider show` runs
- **THEN** it SHALL name the chosen model as that client's, as `aigw status` does, and not the stored selection
- **AND** it SHALL save nothing and write no client file.

#### Scenario: The removal guards see the chosen model as the selection

- **WHEN** a command would remove the chosen model, its provider, or the protocol or endpoint that the choice runs
  on, with or without `--dry-run`
- **THEN** it SHALL be refused with the client and the model named and a step that selects a model for the client
- **AND** configuration and client files SHALL remain unchanged
- **AND** removing the model that the stored selection named before SHALL record the choice in the same transaction,
  name no step back to a model that is gone, and leave the client's file as it is while its Token is missing
- **AND** a withdrawal of that client and an explicit selection for it SHALL NOT be refused for the choice
- **AND** while a file of the client is refused, so that the choice cannot be recorded, removing the model that the
  stored selection names SHALL be refused with that file's own refusal and its step, and not with a validation
  message about the removed model.

#### Scenario: A command changes the settings of a client whose provider has no Token

- **GIVEN** the operator selected in Hermes a model whose provider has no Token that AIGW can use
- **WHEN** a command changes another setting of that client, such as `aigw client edit hermes --via` or `--target`
- **THEN** an edit that drops one of the client's configuration files SHALL withdraw AIGW's settings and record
  from each dropped file, which needs no Token, and SHALL end as it ends with the Token
- **AND** an edit that names the files that the client has, or adds one, SHALL be saved on the chosen model, which
  the command's result names, and the client's file SHALL be left as it is: nothing is dropped, and AIGW's record
  of each file follows what the file shows without a Token
- **AND** the synchronization after the Token is stored SHALL write the file with what the edit saved.

#### Scenario: Another file of the client cannot be written

- **WHEN** a command that names nothing of the client saves while another of that client's files is missing or
  refused
- **THEN** the command SHALL succeed as it does without the choice
- **AND** it SHALL leave the choice in the client's file alone, with the stored selection and AIGW's record of the
  file as they were
- **AND** it SHALL do so whether or not the chosen model's provider has a Token that AIGW can use.

#### Scenario: A command that names the client meets a file that is refused

- **GIVEN** a client's file chose another added model while another of that client's files is missing, has no
  readable record, was edited outside, or selects a different model
- **WHEN** a command that names that client runs, such as `aigw sync` or `aigw client edit` for it
- **THEN** the command SHALL be refused with the step that the refused file gives, and nothing SHALL change
- **AND** it SHALL be refused alike whether or not the client's provider has a Token that AIGW can use.

#### Scenario: The record follows the file at every commit

- **GIVEN** a client's file selects a model that AIGW's record of that file does not name, and nothing else that
  AIGW owns in the file changed
- **WHEN** any command commits, whether or not it names that client or writes that client's file
- **THEN** AIGW's record of the file SHALL name the model that the file selects when the command ends
- **AND** this SHALL hold when the file selects the model that the configuration already stores, as after a
  selection that waited for its Token while the operator picked that model in the client
- **AND** a later pick of the model that the record named before SHALL be followed as a choice.

#### Scenario: Any command after a choice

- **WHEN** a command that neither selects a model for that client nor rolls the configuration back runs after such
  a choice
- **THEN** it SHALL NOT fail or refuse because of the choice, at any depth of `aigw check`
- **AND** a check of every client that passes SHALL write its verified snapshot.

#### Scenario: A client is withdrawn after a choice

- **GIVEN** AIGW's record of each of the client's files is in place
- **WHEN** the operator runs `aigw client disable` for that client, or `aigw self uninstall`
- **THEN** AIGW SHALL withdraw its settings from the client's file, the selection included, as from a file that shows
  the recorded model
- **AND** it SHALL do so whether or not the chosen model's provider has a Token that AIGW can use, and remove its
  record of the file.

#### Scenario: Other managed settings changed

- **WHEN** a managed Hermes provider entry, endpoint or credential command, the address or mode of Hermes's own
  selection, or Codex's provider selection, provider table, scheduler, catalog or credential command changed outside
  AIGW, with or without a model change
- **THEN** no command SHALL follow that client's model
- **AND** a command that would write that client's files SHALL refuse without changing configuration or client
  files.

## ADDED Requirements

### Requirement: A Token that its store cannot observe makes its clients wait

Every command SHALL judge a provider's Token as usable, absent, or not observable, in one place. A Token that cannot
be observed SHALL count as an absent one for each client that uses it: a command SHALL end as it ends when that Token
is absent, so a command that changes several clients SHALL NOT be refused for it, and the result SHALL name the
store's failure and the fix that the failure names for itself.

#### Scenario: Another provider's Token file is refused

- **GIVEN** Hermes selects a model of one provider and AIGW has models of a second provider whose Token file the file
  store refuses
- **WHEN** a command synchronizes Hermes
- **THEN** Hermes SHALL be written for its selection without the second provider's models, as for a provider without
  a Token
- **AND** AIGW's record of Hermes's file SHALL name the model that the file selects.

#### Scenario: The selected provider's Token cannot be observed

- **GIVEN** the store cannot tell whether the selected provider's Token is there, as a locked keychain cannot
- **WHEN** `aigw sync` or its preview runs
- **THEN** the client SHALL wait for its Token as it does when the Token is absent, the other clients SHALL be
  synchronized, and the result SHALL name the store's failure and its fix
- **AND** the command SHALL end as it ends when that Token is absent.

#### Scenario: A client waits for its Token

- **GIVEN** a selected client's provider has no Token that AIGW can use, absent or not observable
- **WHEN** `aigw sync` runs
- **THEN** the result SHALL say that the client's files wait for its Token and SHALL NOT say that the client
  configuration is aligned.
