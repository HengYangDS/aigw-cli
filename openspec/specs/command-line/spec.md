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

`aigw sync` SHALL select writes through `--dry-run` and output format through
`--json` independently. Successful preview and applied results SHALL expose
their execution mode and the same continuation as human output. Rendering
SHALL NOT own transaction commit or compensation.

#### Scenario: Preview a recovery operation as JSON

- **WHEN** `aigw sync --dry-run --json` succeeds
- **THEN** output SHALL be one JSON document with `dry_run: true`
- **AND** `next_action` SHALL identify the corresponding apply command
- **AND** configuration, credentials and client configuration files SHALL
  remain unchanged.

#### Scenario: Apply a recovery operation with JSON output

- **WHEN** `aigw sync --json` succeeds without `--dry-run`
- **THEN** the planned changes to client configuration files SHALL be applied
- **AND** output SHALL be one JSON document with `dry_run: false`, whose
  `next_action` follows the activation state and is `aigw check` only once a
  client is selected
- **AND** preview plans SHALL NOT be presented as observed per-file results.

#### Scenario: Output fails after a successful recovery commit

- **WHEN** a recovery operation commits and its result cannot be written
- **THEN** it SHALL return the output error and preserve the committed state
- **AND** generic presentation SHALL NOT claim that rollback occurred or will
  occur.

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

### Requirement: Local failures name the failed stage

When an AIGW command fails at a local file operation or while loading its
configuration, its human and JSON diagnostic SHALL name the failed operation
and the one reason that the product prints for its condition, or the
configuration stage that failed. It
SHALL name a failed client file as `~/…`, or as configured outside the home
directory, and SHALL contain no other path than one its Fix acts on, and no
configuration content, credential or internal error.

#### Scenario: A local file operation fails

- **WHEN** a command fails because a file operation returns an
  operating-system error
- **THEN** the diagnostic SHALL name the operation and the one reason that the
  product prints for the condition, the same on every platform:
  `no such file or directory` for a missing path, `permission denied` for a
  mode or an access control, `operation not permitted` for a lock, a sandbox
  or system protection, `not a directory` for a regular file in the way, and
  `broken symbolic link` for a link that leads nowhere
- **AND** an error that no condition describes SHALL print the operating
  system's own error number text
- **AND** it SHALL name a client file as `~/…` inside the home directory, or as
  configured outside it, and SHALL contain no other path than one its Fix acts
  on.

#### Scenario: A regular file stands where a folder is needed

- **WHEN** a file operation fails because its path runs through a regular file
  or through a link that leads nowhere
- **THEN** the diagnostic SHALL give the reason `not a directory`, or
  `broken symbolic link`, on every platform, and its Fix SHALL name that path
- **AND** the Fix for a missing folder SHALL name the folder to create, or the
  file itself when AIGW does not create that file, so that following it lets
  the same command run.

#### Scenario: The configuration cannot be parsed

- **WHEN** the configuration is not valid TOML
- **THEN** the diagnostic SHALL name the parse stage with its line and column
- **AND** it SHALL NOT contain configuration content.

#### Scenario: The configuration fails validation

- **WHEN** the configuration does not satisfy the running AIGW's schema
- **THEN** the diagnostic SHALL name the validate stage and the running AIGW
  version
- **AND** its Fix SHALL name `aigw config path` and, for a configuration that
  a newer AIGW wrote, the installed AIGW.

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
- **THEN** its `schema_version` SHALL be the highest of theirs plus one, 3 for
  `aigw check` and 2 for `aigw model search`
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

### Requirement: A flag error names the command's help

A command SHALL refuse a flag that it does not accept, a flag without its value
and a flag value that does not parse before it runs, and the refusal SHALL name
the help of that command as its next action. The credential helper SHALL keep
the refusal that its own failure scenario defines, and any other command that
help does not list, such as `aigw help` itself, SHALL name top-level help.

#### Scenario: A command receives a flag that it does not accept

- **WHEN** `aigw status --not-a-flag` runs
- **THEN** AIGW SHALL refuse the flag, name `aigw status --help` as the next
  action and exit 1.

#### Scenario: The help command receives a flag that it does not accept

- **WHEN** `aigw help setup --not-a-flag` runs
- **THEN** AIGW SHALL refuse the flag, name `aigw --help` as the next action
  and exit 1.

### Requirement: The command tree has twelve top-level commands

AIGW SHALL offer exactly 12 top-level commands, `status`, `use`, `check`,
`model`, `provider`, `client`, `setup`, `sync`, `config`, `self`, `completion`
and `version`, which root help SHALL group as Daily, Models, Providers, Clients
and Maintain. Bare `aigw` SHALL act as `aigw status`, a removed command SHALL
be refused as unknown, and the hidden `aigw credential <client> <fingerprint>`
SHALL stay unchanged.

#### Scenario: Root help shows the tree

- **WHEN** `aigw --help` runs
- **THEN** it SHALL list those 12 commands in those groups and no other command
- **AND** the command groups SHALL offer exactly these subcommands: `model`
  list, search, show, add, edit and remove; `provider` list, show, add, edit,
  rotate, balance and remove; `client` list, enable, disable and edit; `config`
  path, export and rollback; and `self` install, update, rollback, uninstall
  and status.

#### Scenario: A removed command is used

- **WHEN** `aigw doctor` runs
- **THEN** AIGW SHALL refuse the unknown command, name `aigw --help` and exit 1
- **AND** it SHALL read no Token and write nothing.

#### Scenario: Bare aigw runs

- **WHEN** `aigw` runs without arguments
- **THEN** it SHALL write what `aigw status` writes
- **AND** without configuration it SHALL name `aigw setup` and exit 0.

#### Scenario: A client file calls the credential command

- **WHEN** a client configuration file runs
  `aigw credential <client> <fingerprint>`
- **THEN** the command SHALL keep its command line and behavior
- **AND** root help SHALL NOT list it.

#### Scenario: The version is requested

- **WHEN** `aigw version` runs
- **THEN** it SHALL print the running AIGW version and exit 0.

### Requirement: A model argument is a provider/model name

A model argument SHALL be `provider/model`: the text before the first `/`
names the provider, and the rest is that provider's own model ID, byte for
byte. A command that needs an added model, a provider or a client SHALL refuse
an unknown one with the closest matches before it reads a Token or sends a
request, and a model argument without `/` SHALL list the models it could mean.

#### Scenario: A model ID contains a slash

- **WHEN** a command receives `aihubmix/openai/gpt-oss`
- **THEN** it SHALL resolve provider `aihubmix` and model ID `openai/gpt-oss`.

#### Scenario: A model argument has no provider

- **WHEN** `aigw model show gpt-6.1-sol` runs, and DMXAPI and UCloud each have
  that model
- **THEN** it SHALL refuse and list `dmxapi/gpt-6.1-sol` and
  `ucloud/gpt-6.1-sol`.

#### Scenario: A name is unknown

- **WHEN** `aigw provider show dmxap` runs and provider `dmxapi` exists
- **THEN** it SHALL refuse, name `dmxapi` as the closest match and exit 1
- **AND** it SHALL read no Token and send no request.

### Requirement: Flags share one vocabulary

A flag name SHALL mean the same everywhere: `--provider` a provider ID,
`--client` a client ID, `--model` a `provider/model`, `--protocol` `anthropic`,
`responses` or `chat`, `--depth` an evidence depth, `--family` a family ID,
`--capability` a capability, `--endpoint` `<protocol>=<url>`, `--target` a
client's configuration file and `--path` the program file. A removed flag SHALL
be refused like any unknown flag, with no hidden spelling.

#### Scenario: A removed flag is used

- **WHEN** a script runs `aigw check --for codex`
- **THEN** the command SHALL refuse the unknown flag, name `aigw check --help`
  as its Fix and exit 1
- **AND** it SHALL read no Token and write no configuration or client file.

#### Scenario: A protocol has another spelling

- **WHEN** a command receives `--protocol openai_responses`
- **THEN** it SHALL refuse the value and list `anthropic`, `responses` and
  `chat`.

#### Scenario: A filter repeats

- **WHEN** `aigw model list --provider dmxapi --provider ucloud` runs
- **THEN** it SHALL list the models of both providers and no other.

#### Scenario: A query narrows a list

- **WHEN** `aigw model list sonnet 5-5` runs
- **THEN** it SHALL list only the models whose ID or label contains both
  words, ignoring case.

#### Scenario: The program file is named

- **WHEN** `aigw self install --path <file>` runs
- **THEN** AIGW SHALL install the program at that file.

### Requirement: Commands prompt only in a terminal and without JSON

With `--json`, or without a terminal, no command SHALL prompt, and a missing
input SHALL be refused before any change, naming the flag, argument or command
that supplies it. A `--dry-run` preview SHALL send no authenticated request, so
it never validates a Token.

#### Scenario: A script omits a Token

- **WHEN** `aigw provider add dmxapi --endpoint responses=<url>` runs without a
  terminal and without `--token-stdin`
- **THEN** it SHALL refuse, naming `--token-stdin`, and write nothing.

#### Scenario: JSON is requested in a terminal

- **WHEN** `aigw provider rotate dmxapi --json` runs in a terminal without
  `--token-stdin`
- **THEN** it SHALL refuse, naming `--token-stdin`, instead of prompting.

#### Scenario: A preview meets a missing Token

- **WHEN** a `--dry-run` preview needs a Token that is missing
- **THEN** it SHALL report the missing Token without prompting for it or
  sending a request.

### Requirement: Read-only commands remain non-interactive

Read-only commands SHALL NOT prompt, mutate configuration, read secret values
unless authenticating a declared probe, or rewrite a client configuration file.
They SHALL NOT start a client, except `aigw check` at the `client` depth, which
runs each client privately.

#### Scenario: Credential metadata is inaccessible

- **WHEN** a read-only command cannot observe credential metadata
- **THEN** it reports the backend boundary and recovery action
- **AND** does not open an operating-system credential prompt.

### Requirement: Output speaks the product's vocabulary

Help, human output and JSON keys SHALL use the words provider, model, family,
client, protocol and Token, and SHALL NOT use Account, Route, projection,
binding, adapter, admitted or candidate. Keychain item labels SHALL keep their
spelling, because credential storage does not change.

#### Scenario: A retired word would appear

- **WHEN** any command writes its help, human output or a JSON document
- **THEN** no help or human output SHALL contain a retired word, and no JSON key
  SHALL be one, except in a Keychain item label.

### Requirement: Arguments are checked before any change

Every command SHALL check its positional arguments, required flags, flag
combinations and argument shapes before it takes the configuration lock, reads
a Token, sends a request or runs its operation. A refusal SHALL name the
missing argument or flag, or the command's help, never an unrelated command.

#### Scenario: Self update receives invalid archive options

- **WHEN** `aigw self update` receives a blank `--archive` or `--checksums`
  path, or one of them without the other
- **THEN** it SHALL refuse before creating configuration storage or updating.

#### Scenario: Setup receives invalid team file arguments

- **WHEN** `aigw setup --from` receives a blank path, `--token-stdin` without
  `--provider`, or a blank or invalid `--provider`
- **THEN** setup SHALL refuse before creating configuration storage, reading
  the team file or prompting.

#### Scenario: A client command receives invalid arguments

- **WHEN** `aigw client enable` or `aigw client edit` names an unknown client,
  or receives a blank `--executable`, `--target` or `--via`
- **THEN** the command SHALL refuse before creating configuration storage,
  reading credentials or discovering clients.

#### Scenario: A provider or model command receives invalid arguments

- **WHEN** `aigw provider add` receives an invalid provider ID or no valid
  `--endpoint <protocol>=<url>`, `aigw provider edit` only one of
  `--balance-token-stdin` and `--balance-user-id`, or a model command a
  malformed `provider/model`
- **THEN** it SHALL refuse before configuration, lock or credential access.

#### Scenario: An edit names nothing to change

- **WHEN** `aigw provider edit`, `aigw model edit` or `aigw client edit` runs
  without a flag that changes something, or with a blank provider label or
  endpoint
- **THEN** it SHALL refuse before configuration access, naming its help
- **AND** `aigw model edit <provider/model> --label ""` SHALL still clear the
  model's label.

### Requirement: Every failure names one fix

Every failure SHALL say what happened as its Problem and give one Fix: an exact
command, or, when the user must choose a value, a command with placeholders
that JSON carries as `next_detail`, so `next_action` stays exact. A Fix SHALL
NOT be unrelated to the cause or a command that would fail the same way, and a
failure SHALL NOT add generic impact text; impact that adds a specific fact MAY
stay.

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

### Requirement: Every leaf command shows examples

The help of every leaf command SHALL show one to three examples, each a
complete `aigw` command line for that leaf command.

#### Scenario: Read a leaf command's help

- **WHEN** `aigw model search --help` runs
- **THEN** it SHALL show between one and three examples, each starting with
  `aigw model search`.

### Requirement: Arguments complete from local configuration

Shell completion SHALL complete every positional argument and filter flag that
names something configured, from local configuration and without network or
Token access: clients, provider IDs, `provider/model` names, families and
capabilities, while protocols and depths complete from fixed lists. A new name,
such as that of `aigw provider add <provider>`, and a free-text query SHALL get
no completion.

#### Scenario: Complete a model for a client

- **WHEN** the shell completes `aigw use codex` followed by a model
- **THEN** it SHALL offer only the `provider/model` names that Codex can use
- **AND** SHALL NOT offer a model whose provider has no endpoint for a
  protocol that Codex speaks.

#### Scenario: A new name or a query gets no completion

- **WHEN** the shell completes the argument of `aigw provider add` or the
  query of `aigw model search`
- **THEN** it SHALL offer nothing.

#### Scenario: Complete a depth or a protocol

- **WHEN** the shell completes the value of `aigw check --depth` or of
  `aigw model list --protocol`
- **THEN** it SHALL offer `local`, `endpoint`, `model` and `client`, or
  `anthropic`, `responses` and `chat`.

#### Scenario: Complete while the credential store is locked

- **WHEN** completion runs while the credential store is locked or failing
- **THEN** it SHALL return at once with suggestions from local configuration
- **AND** SHALL read no Token, send no request and print nothing else.

### Requirement: Every write has a preview that leaves nothing behind

Every command that writes configuration, client configuration files or the
program SHALL accept `--dry-run`, except guided `aigw setup`, which offers no
preview as `progressive-team-onboarding` defines. A preview SHALL report its
plan with `dry_run: true`, read only what the plan needs, send no authenticated
request, download nothing, run no other program, and leave no file, credential
or Keychain change behind.

#### Scenario: Removing a provider is previewed

- **WHEN** `aigw provider remove <provider> --dry-run` runs
- **THEN** it SHALL report the models, Token items, balance credentials and
  recommendations that the removal affects
- **AND** the configuration and the credential store SHALL be unchanged.

#### Scenario: A preview refuses what its write refuses

- **WHEN** `aigw sync --dry-run` meets a configuration transition that
  `aigw sync` refuses
- **THEN** the preview SHALL refuse it with the error the write gives
- **AND** it SHALL report no plan.

### Requirement: A command's exit status follows its outcome

A command SHALL exit 0 when it succeeds and 1 when it fails or refuses, with or
without `--json`. A blocked preview SHALL write its own document with
`ok: false` and exit 1. `aigw check` and `aigw status` SHALL exit as
`cli-readiness` defines, and `aigw setup` as `progressive-team-onboarding`
defines, so a deferred Token never fails it. A command group without a
subcommand SHALL print its help and exit 0.

#### Scenario: A model search runs without configuration

- **WHEN** `aigw model search --json` runs without configuration
- **THEN** it SHALL exit 1, as human output does.

#### Scenario: A removal preview is blocked

- **WHEN** `aigw provider remove <provider> --dry-run --json` finds a client
  that uses one of the provider's models
- **THEN** the document SHALL have `ok: false` and the blocked status
- **AND** the command SHALL exit 1.

#### Scenario: A command group runs without a subcommand

- **WHEN** `aigw config` runs alone
- **THEN** it SHALL print the group's help and exit 0
- **AND** `aigw help config` SHALL print the same help.

#### Scenario: A command group meets an unknown subcommand

- **WHEN** `aigw model bogus` runs
- **THEN** it SHALL refuse, name `aigw model --help`, and exit 1.
