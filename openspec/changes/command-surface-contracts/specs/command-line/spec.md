## ADDED Requirements

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
- **AND** its references SHALL be named `account`, `route`, `client`, `model`
  or `upstream_model`
- **AND** no list SHALL be null.

#### Scenario: The next step needs a value that AIGW cannot choose

- **WHEN** a client has no Route and AIGW cannot choose one
- **THEN** `next_action` SHALL be absent
- **AND** `next_detail` SHALL show the command with a placeholder for the
  Route.

#### Scenario: A document's fields change

- **WHEN** a release changes the fields of a command's document
- **THEN** that command's `schema_version` SHALL increase
- **AND** the release notes SHALL name each changed field and its
  replacement.

### Requirement: Every result-reporting command offers JSON

Every command that reports a result SHALL accept `--json` and write exactly one
JSON document to standard output. A command whose output is a foreign format,
such as a shell completion script, an exported manifest or a credential
protocol, SHALL NOT accept `--json`.

#### Scenario: A script verifies a client

- **WHEN** `aigw verify --for codex --json` completes
- **THEN** standard output SHALL contain one JSON document with `ok` and the
  verified client's result.

#### Scenario: A foreign format is requested as JSON

- **WHEN** `aigw completion zsh --json` runs
- **THEN** the command SHALL refuse the flag.

### Requirement: Each flag name has one meaning

Across commands, `--for` SHALL name a client or `all`, `--account` an Account,
`--route` a Route, `--model` a canonical Model, `--upstream-model` the model ID
that an endpoint serves, `--protocol` an endpoint protocol, `--target` a
client's native configuration file and `--path` the program file. A renamed
flag SHALL keep its earlier spelling for one release, hidden, with a warning
that names its replacement.

#### Scenario: An earlier flag spelling is used

- **WHEN** a script runs `aigw setup --model gpt-6.1-sol` in the release that
  renames the flag
- **THEN** the command SHALL use the value as `--upstream-model`
- **AND** standard error SHALL name `--upstream-model` as the replacement.

#### Scenario: The program file is named

- **WHEN** `aigw install --path <file>` runs
- **THEN** AIGW SHALL install the program at that file.

### Requirement: Commands without a client scope cover the enabled clients

A command that sends no inference request and runs without `--for` SHALL cover
the enabled clients. `verify`, which may use quota, SHALL require `--for` with
a client or `all`.

#### Scenario: A disabled client has a Route

- **WHEN** `aigw test` runs without `--for` and a disabled client has a
  selected Route
- **THEN** the command SHALL NOT probe that client's endpoint.

#### Scenario: Verify runs without a client

- **WHEN** `aigw verify` runs without `--for`
- **THEN** the command SHALL refuse and name `--for`.

### Requirement: Every write can be previewed

Every command that writes configuration, client files or the program SHALL
accept `--dry-run`. A preview SHALL report its plan with `dry_run: true`, read
only what the plan needs, send no authenticated request, download nothing, run
no other program, and leave no file, credential or Keychain change behind.

#### Scenario: Setup is previewed

- **WHEN** `aigw setup --dry-run` runs with an Account, a Route and a client
- **THEN** it SHALL report the Account, Route and Client Binding that it would
  create
- **AND** it SHALL NOT read, check or store a Token
- **AND** the configuration, the credential store and client files SHALL be
  unchanged.

#### Scenario: Removing a Route is previewed

- **WHEN** `aigw route remove <route> --dry-run` runs
- **THEN** it SHALL report the Client Bindings and recommendations that the
  removal affects
- **AND** the configuration SHALL be unchanged.

### Requirement: Exit status follows the outcome

A command SHALL exit 0 when it succeeds and 1 when it fails or refuses, with or
without `--json`. A command that completes and finds a problem, such as a
client that is not ready or a blocked preview, SHALL write its own document
with `ok: false` and exit 1. A command group without a subcommand SHALL print
its help and exit 0, and `aigw help <command>` SHALL print that command's help.

#### Scenario: A catalog is requested without configuration

- **WHEN** `aigw catalog --json` runs without configuration
- **THEN** it SHALL exit 1, as human output does.

#### Scenario: A rename preview is blocked

- **WHEN** `aigw route rename <old> <new> --dry-run --json` finds a blocking
  reference
- **THEN** the document SHALL have `ok: false` and the blocked status
- **AND** the command SHALL exit 1.

#### Scenario: A command group runs without a subcommand

- **WHEN** `aigw config` runs alone
- **THEN** it SHALL print the group's help and exit 0.

#### Scenario: A command group meets an unknown subcommand

- **WHEN** `aigw route bogus` runs
- **THEN** it SHALL refuse, name `aigw route --help`, and exit 1.

## MODIFIED Requirements

### Requirement: Invocation output has one owner

The CLI root SHALL finalize command output once, preserving command, output and
cleanup failures. Generic JSON errors SHALL use the same Problem and continuation
as human errors, carry the failure envelope's own `schema_version`, and name the
configuration stage when loading the configuration failed. Credential-helper
stdout SHALL contain only credential output.

#### Scenario: A JSON-capable command fails before writing a result

- **WHEN** the selected command has parsed `--json` as true and fails before
  writing its result
- **THEN** stdout SHALL contain one JSON document with `schema_version`,
  `ok: false`, `error` and `next_action` or `next_detail`, preserving available
  evidence and impact
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
- **AND** stale projection rejection SHALL precede credential access and explain
  that the client must reload its synchronized configuration
- **AND** diagnostic write failures SHALL remain observable to the caller.
