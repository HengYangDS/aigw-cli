# command-output Specification

## Purpose

Define what a command prints: who owns invocation output, how human output is laid out and named, and
the one contract of every JSON document.

## Requirements

### Requirement: Invocation output has one owner

The CLI root SHALL finalize command output once, preserving command, output and
cleanup failures. Generic JSON errors SHALL use the same Problem and Fix as
human errors, carry the failure envelope's own `schema_version`, and name the
configuration stage when loading the configuration failed. Credential-helper
stdout SHALL contain only credential output.

#### Scenario: A JSON-capable command fails before writing a result

- **WHEN** the selected command has parsed `--json` as true and fails before
  writing its result
- **THEN** stdout SHALL contain one JSON document with `schema_version`,
  `ok: false`, `error` and `next_action` or `next_detail`, preserving available
  evidence and any specific impact
- **AND** the command SHALL return a nonzero exit status.

#### Scenario: The configuration cannot be loaded

- **WHEN** a JSON-capable command fails while reading, parsing or validating
  the configuration
- **THEN** the failure document SHALL name `stage` as `configuration_read`,
  `configuration_parse` or `configuration_validate`
- **AND** a parse failure SHALL carry `line` and `column`.

#### Scenario: Output format follows the parsed command

- **WHEN** `--json=false` is supplied, or `--json` appears after the `--`
  argument separator
- **THEN** the generic error SHALL use human output
- **AND** command-resolution failures before flag parsing SHALL use human output.

#### Scenario: Output cannot be completed

- **WHEN** the result writer fails or accepts only part of a write
- **THEN** the invocation SHALL preserve the write failure and any command error
- **AND** SHALL NOT append another result to the incomplete output
- **AND** a subsequent invocation SHALL start with independent output state.

#### Scenario: Credential helper fails

- **WHEN** the credential helper fails command validation or execution
- **THEN** the executable SHALL return a nonzero exit status without rendering
  a diagnostic on credential stdout
- **AND** stderr SHALL explain the failure and its recovery using safe,
  product-owned diagnostics, without echoing raw backend errors, configuration
  values or credential bytes, including failures during initialization
- **AND** the refusal of a fingerprint from a stale client configuration file
  SHALL precede credential access and explain that the client must reload its
  synchronized configuration
- **AND** diagnostic write failures SHALL remain observable to the caller.

### Requirement: Terminal layout follows semantic fields and display width

Human output SHALL use one presentation owner for terminal-cell measurement,
ANSI-aware layout, wrapping, and aligned semantic fields. Groups stack when
content cannot fit; metadata SHALL own executable names and pflag SHALL own
option grammar. Color SHALL NOT alter alignment. Machine, helper, and interactive
contracts SHALL remain unchanged. Copyable commands SHALL preserve characters,
whitespace, and logical lines; only explanatory text MAY wrap, without adding
shell syntax.

#### Scenario: Copy an executable command from a narrow terminal

- **WHEN** an executable example or follow-up command exceeds the terminal width
- **THEN** its output SHALL preserve the command bytes apart from presentation
  indentation, optional styling and the final output newline
- **AND** neither quoted whitespace nor argument tokens SHALL be rewritten to
  satisfy an explanatory-text width rule.

#### Scenario: Starting commands have different lengths

- **WHEN** root help presents the setup, selection and readiness journey
- **THEN** command and explanation SHALL be distinct fields with one description
  column derived from the group's longest label
- **AND** a renamed root command SHALL propagate without hard-coded product text.

#### Scenario: A terminal is narrow or a value spans multiple lines

- **WHEN** human output contains long labels, unbroken values or multiple lines
- **THEN** the renderer SHALL retain text and continuation indentation within the
  available display width and use stacked rows where necessary
- **AND** colored and plain output SHALL preserve the same layout.

#### Scenario: Inspect the complete public command tree

- **WHEN** every public command renders help at supported narrow and wide widths
- **THEN** headings, descriptions, usage grammar and options SHALL fit those widths
- **AND** native option meaning and credential-free help SHALL remain intact.

### Requirement: Human output groups share one value column

A command SHALL submit each group of fields, with their state symbols and
details, as one unit. The group SHALL share one value column at least two cells
after its longest label and symbol; values and details continue under it, and
the group stacks as a whole only when too little width remains for values.
Standalone notes SHALL keep one indentation at every width, and explanatory
text SHALL break only at spaces unless one word exceeds the width.

#### Scenario: Classified rows have labels of different lengths

- **WHEN** a section such as the local checks of `aigw check --verbose` lists
  rows whose labels differ in length or exceed the default label column
- **THEN** every value of the group SHALL start in one column at least two
  cells after the longest label and its state symbol
- **AND** a value or detail wider than the remaining width SHALL continue under
  that column instead of stacking its row alone.

#### Scenario: Human output has no known width

- **WHEN** human output is piped or the terminal width is unknown
- **THEN** groups and notes SHALL keep the same columns and indentation as at a
  known width, without wrapping.

#### Scenario: Wrapped text contains hyphenated identifiers

- **WHEN** explanatory text containing client, provider or model identifiers
  wraps
- **THEN** lines SHALL break only at spaces, keeping each identifier whole
  unless it alone exceeds the available width.

#### Scenario: Help lists command options

- **WHEN** command help lists its options
- **THEN** option lines SHALL start at the indentation of the other section
  entries and keep pflag's own layout.

### Requirement: Human output names clients and protocols by their labels

Human output SHALL name each client by its registry display label and each
protocol by its protocol label, in rows, prose and problems alike, and SHALL
NOT capitalize an identifier into a third name. Help SHALL list the
identifiers that a flag or argument accepts, and copyable commands SHALL keep
identifiers.

#### Scenario: A client with a multi-word label appears in human output

- **WHEN** `status`, `use`, `check`, `model`, `provider`, `client`, `setup` or
  `sync` output names Claude Desktop
- **THEN** it SHALL use `Claude Desktop`
- **AND** JSON output SHALL keep the identifier `claude-desktop`.

#### Scenario: Help lists client choices

- **WHEN** help describes a `<client>` argument or the `--client` flag
- **THEN** it SHALL list the client identifiers that it accepts.

#### Scenario: Human output shows a protocol

- **WHEN** `aigw use` or `aigw sync` output shows a client's protocol
- **THEN** it SHALL use the protocol label, such as `OpenAI Responses` for
  `responses`.

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

### Requirement: Every result-reporting command offers JSON

Every command that reports a result SHALL accept `--json` and write exactly one
JSON document to standard output. A command whose output is a foreign format,
such as a shell completion script, an exported team file or a credential
protocol, SHALL NOT accept `--json`.

#### Scenario: A script verifies a client

- **WHEN** `aigw check codex --depth client --json` completes
- **THEN** standard output SHALL contain one JSON document with `ok` and
  Codex's result, including the depth it reached.

#### Scenario: A foreign format is requested as JSON

- **WHEN** `aigw completion zsh --json` runs
- **THEN** the command SHALL refuse the flag.

### Requirement: Output speaks the product's vocabulary

Help, human output and JSON keys SHALL use the words provider, model, family,
client, protocol and Token, and SHALL NOT use Account, Route, projection,
binding, adapter, admitted or candidate. Keychain item labels SHALL keep their
spelling, because credential storage does not change.

#### Scenario: A retired word would appear

- **WHEN** any command writes its help, human output or a JSON document
- **THEN** no help or human output SHALL contain a retired word, and no JSON key
  SHALL be one, except in a Keychain item label.
