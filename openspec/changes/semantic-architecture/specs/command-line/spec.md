## Purpose

Define the command line's shared behavior: who owns invocation output, how
arguments are admitted before any change, how help and local failures are
presented, and how human output is laid out.

## ADDED Requirements

### Requirement: Quiet handled failures

Handled CLI failures MUST NOT emit a framework usage banner, warning,
traceback, or false completion message.

#### Scenario: A handled CLI failure occurs

- **WHEN** a command returns an expected operational error
- **THEN** the command SHALL return that error without usage, warning,
  traceback, or completion residue

### Requirement: Invocation output has one owner

The CLI root SHALL finalize command output once, preserving command, output and
cleanup failures. Generic JSON errors SHALL use the same Problem and continuation
as human errors. Credential-helper stdout SHALL contain only credential output.

#### Scenario: A JSON-capable command fails before writing a result

- **WHEN** the selected command has parsed `--json` as true and fails before
  writing its result
- **THEN** stdout SHALL contain one JSON document with `ok: false`, `error` and
  `next_action`, preserving available evidence and impact
- **AND** the command SHALL return a nonzero exit status.

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

### Requirement: Native warning symbols are diagnostics

A native diagnostic line whose first visible character is an explicit warning
or error symbol, `⚠`, `✗`, `✖`, `✘` or `❌`, SHALL count as a warning in
every check that rejects explicit native warnings. Check marks, progress
glyphs and symbols inside a line SHALL NOT.

#### Scenario: Hermes reports an out-of-sync installation

- **WHEN** Hermes exits zero after printing a line that starts with `⚠`
- **THEN** verification SHALL report incomplete qualification without the
  private diagnostic.

#### Scenario: A client prints ordinary progress

- **WHEN** a client prints `session_id:`, a `✓` result or a `↻` resume line
- **THEN** that line SHALL NOT be a warning.

### Requirement: Diagnostics name the native file they inspect

When doctor reports an observation about one native client configuration file
under the user's home directory, human and JSON output SHALL name that file by
its path relative to the home directory. A file outside the home directory
SHALL NOT be named, and its human label SHALL say that it lies outside the home
directory. Diagnostics SHALL NOT expose an absolute or other private path.

#### Scenario: Codex projects into its default home

- **WHEN** doctor inspects a Codex binding whose target is
  `~/.codex/config.toml`
- **THEN** the human check is labeled `Codex configuration ~/.codex/config.toml`
- **AND** the JSON check keeps its name `codex:target-1` and carries
  `~/.codex/config.toml` as `target`.

#### Scenario: A Codex target lies outside the home directory

- **WHEN** a Codex binding's target lies outside the home directory
- **THEN** the human check is labeled by its position and says that it lies
  outside the home directory
- **AND** neither human nor JSON output contains the file's path.

### Requirement: Diagnostic assertions read text, not terminal presentation

Tests and gates that assert a native tool's diagnostics SHALL judge the
diagnostic text after removing terminal escape sequences. A hosted runner that
colors the output SHALL reach the same judgement as an uncolored local
terminal.

#### Scenario: A hosted runner colors a native tool's warning

- **WHEN** a native tool prints an expected warning with ANSI escape sequences
  between its words
- **THEN** the assertion SHALL count that warning exactly once
- **AND** the same output without escape sequences SHALL yield the same count.

### Requirement: Doctor knows a target on another volume is outside home

When the home directory and a target are absolute paths on different volumes,
doctor SHALL label the target as outside the home directory. A relative or
invalid home directory SHALL leave the target's location unknown.

#### Scenario: A Windows target is on another drive or share

- **WHEN** HOME is on one drive and the Codex target is on another drive or a
  UNC share
- **THEN** doctor SHALL label the target as outside the home directory.

#### Scenario: Volume names differ only in letter case

- **WHEN** both paths name the same drive in different letter case
- **THEN** doctor SHALL treat them as one volume.
