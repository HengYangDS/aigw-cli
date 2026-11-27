## MODIFIED Requirements

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
- **THEN** the client's row SHALL name that model, and the check SHALL pass for it, unless the choice ended a
  setting that the client's file still holds (the next scenario)
- **AND** the output SHALL name the followed selection, also when the check fails for another reason
- **AND** configuration and client files SHALL remain unchanged.

#### Scenario: The choice ended a forwarding that Hermes's file still holds

- **GIVEN** the stored selection of Hermes forwards, and the operator selected in Hermes a model whose upstream the
  forwarding grammar cannot name, such as a model on another protocol than Responses
- **WHEN** `aigw status` or `aigw check hermes --depth local` runs
- **THEN** the output SHALL name the followed selection with `forwarding_cleared`
- **AND** the check SHALL report that Hermes's file differs from the selected model, with `aigw sync` as its step,
  because the file still holds the forwarding address
- **AND** `aigw sync` SHALL then write the file without the forwarding and save the choice, and a following check
  SHALL pass.

#### Scenario: The fields of an entry

- **WHEN** a JSON document names a followed selection
- **THEN** its entry SHALL carry `client`, `model` and `earlier`, and for a client that speaks several protocols
  `protocol` and `earlier_protocol`
- **AND** it SHALL carry `returns_with`, the command that selects the earlier model again, while the configuration
  holds that model and a shell passes its ID unquoted
- **AND** `forwarding_cleared` SHALL be true when the client's forwarding ended because the grammar cannot name the
  followed model's upstream, and `retries_cleared` when its retry budgets ended with the earlier provider, and human
  output SHALL say so.

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

### Requirement: An explicit selection replaces the client's own

`aigw use <client> <provider/model>` for Codex or Hermes SHALL write the named model whatever model the client's own
selection names, a model that AIGW has not added included, when nothing else that AIGW owns in the client's file
changed.

#### Scenario: An explicit selection names another model

- **WHEN** the operator runs `aigw use <client> <provider/model>` for Hermes or Codex after choosing a different
  model in that client
- **THEN** AIGW SHALL write the named model, not the client's choice.

#### Scenario: An explicit selection returns to the stored model

- **GIVEN** the stored selection forwards to a local address or carries retry budgets, and the operator then picked
  a model of another provider in the client
- **WHEN** `aigw use` selects the stored model again
- **THEN** the forwarding and the retry budgets of the stored selection SHALL be kept
- **AND** the result SHALL NOT report them as cleared
- **AND** without `--protocol` the selection SHALL keep the stored protocol, not the protocol of the model that the
  client chose in between.

#### Scenario: An explicit selection of another upstream after a choice

- **GIVEN** the stored selection forwards to a local address, and the operator then picked another model in the
  client
- **WHEN** `aigw use` selects a model of another upstream for that client
- **THEN** the forwarding SHALL stay with its destination following that upstream, or the selection SHALL be refused
  naming `aigw client edit <client> --direct` when the grammar cannot name it, as without the choice.

#### Scenario: An explicit selection over a model that AIGW has not added

- **WHEN** the client's selection names a model that AIGW has not added and the operator runs
  `aigw use <client> <provider/model>`
- **THEN** AIGW SHALL write the named model and the client's files SHALL match the selection.

#### Scenario: Hermes's selection has an edited address

- **WHEN** the address or mode of Hermes's own selection differs from its provider entry and the operator runs
  `aigw use hermes <provider/model>`
- **THEN** AIGW SHALL refuse as for any other managed setting changed outside AIGW.
