# command-line Specification

## Purpose

Define the command line's shared behavior: its command tree and flags, how
arguments are admitted before any change, how help and failures are presented,
and what a command's exit status means.

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

- **WHEN** `aigw client enable` or `aigw client edit` names an unknown client, receives a blank `--program`,
  `--extensions`, `--target` or `--via`, receives both `--program` and `--extensions`, or receives `--extensions` for
  Claude Desktop or Hermes
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
