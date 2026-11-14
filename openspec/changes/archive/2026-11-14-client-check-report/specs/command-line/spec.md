## MODIFIED Requirements

### Requirement: Command JSON documents follow one contract

Every JSON document that a command writes SHALL carry the command's integer
`schema_version` and `ok`. An object's own identifier SHALL be `id`, and a
reference to another object SHALL be named after its type. Lists SHALL be
arrays, never null, and states SHALL be snake_case. `next_action` SHALL be a
complete `aigw` command that a script can run, or absent, and other guidance
SHALL be `next_detail`.

#### Scenario: A script reads a command's document

- **WHEN** a command writes its JSON document
- **THEN** the document SHALL carry `schema_version` and `ok`
- **AND** its fields SHALL use `provider`, `model`, `family`, `client`,
  `protocol` and `depth`, with each `model` written as `provider/model`, and
  SHALL NOT use `account`, `route` or `upstream_model`
- **AND** no list SHALL be null.

#### Scenario: The next step needs a value that AIGW cannot choose

- **WHEN** a client has no selected model and AIGW cannot choose one
- **THEN** `next_action` SHALL be absent
- **AND** `next_detail` SHALL show `aigw use <client> <provider/model>` with
  the client filled in and a placeholder for the model.

#### Scenario: A document's fields change

- **WHEN** a release changes the fields of a command's document
- **THEN** that command's `schema_version` SHALL increase by one
- **AND** the release notes SHALL name each changed field and its
  replacement.

#### Scenario: A command's document replaces others

- **WHEN** a command's document replaces the documents of earlier commands, as
  `aigw check` replaces those of `check`, `doctor`, `test` and `verify`
- **THEN** its `schema_version` SHALL start at the highest of theirs plus one,
  3 for `aigw check` and 2 for `aigw model search`
- **AND** a document without a predecessor SHALL start at 1.

### Requirement: Every failure names one fix

Every failure SHALL say what happened as its Problem and give one Fix: an exact
command; a command with placeholders, when the user must choose a value; or a
sentence on what to change or how to learn it, when no `aigw` command fixes the
cause. JSON SHALL carry the last two as `next_detail`, so `next_action` stays
exact. A Fix SHALL NOT be unrelated to the cause or a command that would fail
the same way, and a failure SHALL NOT add generic impact text; impact that adds
a specific fact MAY stay.

#### Scenario: A provider rejects the Token

- **WHEN** `aigw check` receives HTTP 401 from UCloud for Hermes
- **THEN** Hermes's Problem SHALL say that the provider rejected the Token
- **AND** its Fix SHALL be `aigw provider rotate ucloud`, with no generic
  impact text.

#### Scenario: The Fix needs the user's choice

- **WHEN** a client has no selected model and AIGW cannot choose one
- **THEN** the Fix SHALL be `aigw use <client> <provider/model>` with the
  client filled in and a placeholder for the model
- **AND** JSON SHALL carry it as `next_detail`, with no `next_action`.

#### Scenario: The configuration does not load

- **WHEN** reading, parsing or validating the configuration fails
- **THEN** the Fix SHALL name the failed stage and `aigw config path`
- **AND** it SHALL NOT name `aigw check` or another command that fails the same
  way.

#### Scenario: An argument is missing

- **WHEN** `aigw provider show` runs without its `<provider>` argument
- **THEN** the Problem SHALL name the missing argument
- **AND** the Fix SHALL be `aigw provider show --help`, not `aigw check`.
