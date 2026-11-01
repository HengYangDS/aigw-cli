# command-line Specification

## Purpose

Define the command line's shared behavior: who owns invocation output, how
arguments are admitted before any change, how help and local failures are
presented, and how human output is laid out.

## Requirements

### Requirement: Quiet handled failures

Handled CLI failures MUST NOT emit a framework usage banner, warning,
traceback, or false completion message.

#### Scenario: A handled CLI failure occurs

- **WHEN** a command returns an expected operational error
- **THEN** the command SHALL return that error without usage, warning,
  traceback, or completion residue

### Requirement: Recovery execution and presentation are independent

Synchronization and repair SHALL select writes through `--dry-run` and output
format through `--json` independently. Successful preview and applied results
SHALL expose their execution mode and the same continuation as human output.
Rendering SHALL NOT own transaction commit or compensation.

#### Scenario: Preview a recovery operation as JSON

- **WHEN** `sync` or `repair` succeeds with `--dry-run --json`
- **THEN** output SHALL be one JSON document with `dry_run: true`
- **AND** `next_action` SHALL identify the corresponding apply command
- **AND** configuration, credentials and client projections SHALL remain
  unchanged.

#### Scenario: Apply a recovery operation with JSON output

- **WHEN** `sync` or `repair` succeeds with `--json` without `--dry-run`
- **THEN** the owned projection SHALL be applied
- **AND** output SHALL be one JSON document with `dry_run: false` and
  `next_action: "aigw check"`
- **AND** preview plans SHALL NOT be presented as observed per-target results.

#### Scenario: Output fails after a successful recovery commit

- **WHEN** a recovery operation commits and its result cannot be written
- **THEN** it SHALL return the output error and preserve the committed state
- **AND** generic presentation SHALL NOT claim that rollback occurred or will
  occur.

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

### Requirement: Command help preserves native metadata

Command help SHALL derive grammar, descriptions, examples and option metadata
from the command tree. pflag SHALL own flag notation, value types and declared
defaults. Help SHALL remain available without configuration or credential access.

#### Scenario: Inspect detailed command help

- **WHEN** a command declares detailed text, examples, typed flags, defaults or
  inherited options
- **THEN** help SHALL expose those declarations in the existing terminal layout
- **AND** defaults SHALL reflect declarations rather than previously parsed values
- **AND** narrow output SHALL preserve the same option meaning.

#### Scenario: Inspect any command before setup

- **WHEN** any command is invoked with `--help` before configuration exists
- **THEN** help SHALL show its native option metadata without creating
  configuration storage or invoking its operation.

### Requirement: Argument admission precedes mutation

The CLI SHALL validate positional arguments, required flags, flag relationships
and command-specific argument shape before acquiring a configuration mutation
lock or executing an operation. Cobra SHALL own shared flag validation.

#### Scenario: Select a local update with invalid paths or options

- **WHEN** an explicitly supplied candidate or checksum path is empty or
  whitespace-only, its paired option is missing, or rollback is combined with
  candidate options
- **THEN** the command SHALL reject the invocation before creating configuration
  storage or calling an online update, candidate update or rollback operation.

#### Scenario: Supply invalid manifest setup arguments

- **WHEN** the manifest path or explicitly selected Account ID is blank,
  manifest mode is combined with single-route options, JSON output is
  requested without manifest mode, or manifest-mode Token input has no Account
  owner
- **THEN** setup SHALL reject the invocation before creating configuration
  storage, reading the manifest, or executing setup.

#### Scenario: Supply incomplete account finalization intent

- **WHEN** account finalization lacks explicit old/new identifiers, or credential
  rotation confirmation is requested without finalization
- **THEN** argument admission SHALL reject the invocation before acquiring a
  configuration lock or creating configuration storage
- **AND** the diagnostic SHALL direct the operator to that command's help,
  rather than an unrelated readiness check.

#### Scenario: Supply invalid rename arguments

- **WHEN** noninteractive Account, Model, or Route rename omits an identifier, an
  explicitly supplied identifier violates the configuration identifier contract,
  or an incomplete interactive invocation has no prompt capability
- **THEN** argument admission SHALL reject it before configuration access,
  mutation-lock acquisition, credential access or discovery
- **AND** the diagnostic SHALL direct the operator to the invoked command's help
- **AND** complete explicit identifiers SHALL require no prompt, while guided
  rename SHALL remain available when interactive input can complete the request.

#### Scenario: Supply invalid selection or adapter arguments

- **WHEN** noninteractive selection omits its Route, an adapter client is not
  admitted, its executable is missing or blank, or a required target is missing
  or an explicitly supplied target is blank
- **THEN** the command SHALL reject the invocation with an actionable repair
  command before creating configuration storage
- **AND** it SHALL NOT invoke credential access, client discovery or mutation.

#### Scenario: Supply invalid creation or import arguments

- **WHEN** Account, Model, or Route creation supplies an invalid identifier, omits
  required client/model/Account flags, supplies blank required values, selects
  an inadmitted client, or configuration import supplies a blank manifest path
- **THEN** the command SHALL reject the invocation with actionable guidance
  before creating configuration storage or accessing credentials.

#### Scenario: Supply invalid metadata edit intent

- **WHEN** Account, Model, or Route editing supplies no editable flag, an invalid
  identifier or a blank label or endpoint, Route removal supplies an invalid
  identifier, or diagnostic-credential enablement lacks an interactive terminal
- **THEN** admission SHALL reject the invocation before configuration access or
  creation, mutation-lock acquisition or credential access
- **AND** native flag-group failures SHALL identify the invoked command's help,
  not an unrelated readiness check
- **AND** explicit empty Model or Route display metadata SHALL remain a valid
  request to clear that optional field.

### Requirement: Local failures name the failed stage

When an AIGW command fails at a local file operation or while loading its
configuration, its human and JSON diagnostic SHALL name the failed operation
and the operating-system error, or the configuration stage that failed. A parse
failure SHALL name the line and column, and a validation failure SHALL name the
running AIGW version. The diagnostic SHALL NOT contain a path, configuration
content, a credential or an internal error message.

#### Scenario: A local file operation fails

- **WHEN** a command fails because a file operation returns an
  operating-system error
- **THEN** the diagnostic SHALL name the operation and the error
- **AND** it SHALL NOT contain the file path.

#### Scenario: The configuration cannot be parsed

- **WHEN** the configuration is not valid TOML
- **THEN** the diagnostic SHALL name the parse stage with its line and column
- **AND** it SHALL NOT contain configuration content.

#### Scenario: The configuration fails validation

- **WHEN** the configuration does not satisfy the running AIGW's schema
- **THEN** the diagnostic SHALL name the validate stage and the running AIGW
  version
- **AND** its next step SHALL name `aigw doctor` and, for a configuration that
  a newer AIGW wrote, the installed AIGW.

### Requirement: Route presentation groups admitted service types

Route presentation SHALL order by Account presentation rank and identity,
admitted protocol set in native service display order, Model presentation rank
and identity, then Route identity. This applies without explicit ranks.
Multi-interface Routes SHALL use their complete ordered protocol set without
selecting a runtime protocol. Canonical enumeration, exports, and Client
Bindings SHALL remain unchanged.

#### Scenario: A higher-ranked Model belongs to a later service group

- **WHEN** one Account has a Chat Completions Route ranked before a Responses
  Route by Model
- **THEN** human and JSON Route lists SHALL place the Responses group first
- **AND** preserve Model order within each service group.

#### Scenario: Listing does not change routing identity

- **WHEN** ranked, unranked, or multi-interface Routes are listed
- **THEN** repeated human and JSON listings SHALL have the same presentation order
- **AND** preserve stored configuration, canonical enumeration, exports, and
  explicit Client Bindings.

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

- **WHEN** a section such as the doctor checks lists rows whose labels differ in
  length or exceed the default label column
- **THEN** every value of the group SHALL start in one column at least two
  cells after the longest label and its state symbol
- **AND** a value or detail wider than the remaining width SHALL continue under
  that column instead of stacking its row alone.

#### Scenario: Human output has no known width

- **WHEN** human output is piped or the terminal width is unknown
- **THEN** groups and notes SHALL keep the same columns and indentation as at a
  known width, without wrapping.

#### Scenario: Wrapped text contains hyphenated identifiers

- **WHEN** explanatory text containing client, Route or model identifiers wraps
- **THEN** lines SHALL break only at spaces, keeping each identifier whole unless
  it alone exceeds the available width.

#### Scenario: Help lists command options

- **WHEN** command help lists its options
- **THEN** option lines SHALL start at the indentation of the other section
  entries and keep pflag's own layout.

### Requirement: Human output names clients and protocols by their labels

Human output SHALL name each client by its registry display label and each
protocol by its protocol label, in rows, prose and problems alike, and SHALL
NOT capitalize an identifier into a third name. Help SHALL list the
identifiers that a flag accepts, and copyable commands SHALL keep identifiers.

#### Scenario: A client with a multi-word label appears in human output

- **WHEN** check, test, verify, status, sync, repair, migration, selection,
  route or account output names Claude Desktop
- **THEN** it SHALL use `Claude Desktop`
- **AND** JSON output SHALL keep the identifier `claude-desktop`.

#### Scenario: Help lists client choices

- **WHEN** help describes a `--for` flag
- **THEN** it SHALL list the identifiers that the flag admits.

#### Scenario: Human output shows a protocol

- **WHEN** selection or migration output shows a binding's protocol
- **THEN** it SHALL use the protocol label, such as `OpenAI Responses`.

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
