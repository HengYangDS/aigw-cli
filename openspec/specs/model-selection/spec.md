# model-selection Specification

## Purpose

Define how a client uses one provider's model, named `provider/model`:
explicit selection, adding a model from the provider's catalog with its
protocol, family and capabilities, recommendations, forwarding and retry
budgets, checks of a named model and disabling a client, without guessing what
the operator did not say.

## Requirements

### Requirement: A client uses one model

`aigw use <client> <provider/model>` SHALL make that model the client's
selection and SHALL change only that client's selection and the client
configuration files that AIGW owns for it, except that a model or protocol that
it adds SHALL also update the files of other clients that list that provider's
models, and the result SHALL name them. Several clients MAY use one model; each
SHALL keep its own selection, and there SHALL be no global selection.

#### Scenario: Select a model for one client

- **WHEN** the operator runs `aigw use codex dmxapi/gpt-6.1-sol` for a model
  that already declares `responses`
- **THEN** AIGW SHALL update only Codex's selection and the client
  configuration files that AIGW owns for Codex
- **AND** SHALL preserve every other client's selection, files and credentials
- **AND** its next action SHALL be `aigw check codex`.

#### Scenario: An added model reaches the clients that list its provider

- **WHEN** `aigw use codex dmxapi/gpt-6.2` adds that model while Hermes lists
  the `responses` models of DMXAPI
- **THEN** Hermes's client configuration file SHALL list the model in the same
  transaction, and the result SHALL name Hermes
- **AND** Hermes's selection SHALL stay unchanged.

#### Scenario: One model serves several clients

- **GIVEN** one model has protocols that several clients speak
- **WHEN** the operator selects it for each of those clients
- **THEN** AIGW SHALL keep one model and one selection per client
- **AND** SHALL NOT copy the model or introduce a global selection.

### Requirement: Selection prompts only in a terminal

`aigw use` with a missing argument SHALL prompt only in a terminal and without
`--json`, through filterable lists that show recommended models first, and
SHALL otherwise refuse as `command-line` requires of every command.

#### Scenario: A terminal prompts for what is missing

- **WHEN** `aigw use` runs in a terminal with no argument, with only a client,
  or with only a `provider/model`
- **THEN** it SHALL ask for the client and then the model, for the model, or
  for a client that can use the model
- **AND** every model list SHALL be filterable and show recommended models
  first.

### Requirement: Using a model adds it after the catalog check

When `aigw use` or `aigw model add` names a model that has not been added,
AIGW SHALL first read that provider's authenticated catalog and add the model
only if the catalog lists its ID; `aigw use` SHALL add and select it in one
transaction. `--unlisted` SHALL skip the catalog check, and a preview SHALL
read no catalog.

#### Scenario: The catalog lists the model

- **WHEN** `aigw use codex dmxapi/gpt-6.2` names a model that has not been
  added, and the DMXAPI catalog lists `gpt-6.2`
- **THEN** AIGW SHALL add `dmxapi/gpt-6.2` and select it for Codex in one
  transaction
- **AND** the result SHALL report the addition.

#### Scenario: The catalog does not list the model

- **WHEN** the provider's catalog does not list the named model ID
- **THEN** AIGW SHALL refuse before any write and name the closest listed IDs.

#### Scenario: The catalog is unreadable or empty

- **WHEN** the provider's catalog cannot be read or lists no model
- **THEN** AIGW SHALL refuse before any write
- **AND** its Fix SHALL be the same command with `--unlisted`.

#### Scenario: The operator adds an unlisted model

- **WHEN** `aigw use` or `aigw model add` runs with `--unlisted`
- **THEN** AIGW SHALL add the model without reading the provider's catalog.

#### Scenario: Adding a model is previewed

- **WHEN** `aigw use <client> <provider/model>` or `aigw model add` runs with
  `--dry-run` for a model that has not been added
- **THEN** it SHALL read no catalog, report the addition and say that the
  catalog listing was not checked
- **AND** `aigw use` SHALL also report the selection, read no Token and write
  nothing.

### Requirement: The client decides a model's protocol

A model that `aigw use` adds, or an added model with none of the client's
protocols, SHALL gain one of them after the catalog check, and the result SHALL
say so. When several protocols are possible, those the model and the client
share or, for a gain, those the client and the provider share, `aigw use` SHALL
keep the client's current or recommended protocol or require a choice through a
prompt or `--protocol`, and SHALL NOT infer one from a catalog or a name.

#### Scenario: Codex uses a model that lacks its protocol

- **GIVEN** `ucloud/gpt-6.1-sol` declares only `chat`, and UCloud has a
  `responses` endpoint
- **WHEN** the operator runs `aigw use codex ucloud/gpt-6.1-sol`
- **THEN** the model SHALL gain `responses` after the catalog check
- **AND** the result SHALL say that the model gained a protocol.

#### Scenario: The provider serves none of the client's protocols

- **WHEN** the named model's provider has no endpoint for any protocol that
  the client speaks
- **THEN** the command SHALL fail before credential access, network access or
  any write
- **AND** SHALL name the client, the model and the protocols the client speaks.

#### Scenario: Hermes selects a multi-protocol model

- **WHEN** Hermes selects a model with several compatible protocols and no
  current or recommended protocol applies
- **THEN** a selection in a terminal SHALL offer each compatible protocol
- **AND** a non-interactive selection SHALL refuse with the Fix
  `aigw use <client> <provider/model> --protocol <a|b>`, with the client, the
  model and the compatible protocols filled in
- **AND** a failed or incompatible choice SHALL leave every selection
  unchanged.

#### Scenario: A selected protocol requires a new Token

- **WHEN** a selection in a terminal obtains a missing Token for a client with
  several compatible protocols
- **THEN** validation SHALL probe only the chosen protocol and endpoint
- **AND** a failed probe SHALL NOT store the Token or change the selection.

### Requirement: Selection fails closed instead of guessing

AIGW SHALL NOT infer a client, model or capability, or choose among protocols
that a client and a provider share, from identifiers, families, endpoint
presence, another client's selection or declaration order.

#### Scenario: A client has no selected model

- **WHEN** an operation requires a client that has no selected model
- **THEN** AIGW SHALL report that exact missing selection
- **AND** SHALL NOT substitute another client's model.

### Requirement: An added model takes only the family with its ID

A model that `aigw use` or `aigw model add` adds without `--family` SHALL take
an existing family whose ID equals its model ID, ignoring case, and otherwise
SHALL be its own family. AIGW SHALL NOT derive a family from a prefix or any
other part of an ID, so a variant SHALL join a family only through `--family`
or a team file.

#### Scenario: A model takes the family with its ID

- **GIVEN** family `minimax-m3` exists
- **WHEN** `aigw use claude ucloud/MiniMax-M3` adds that model
- **THEN** its family SHALL be `minimax-m3`.

#### Scenario: A variant does not join by its prefix

- **GIVEN** family `gpt-6.1-sol` exists
- **WHEN** `aigw use codex dmxapi/gpt-6.1-sol-cdx` adds that model
- **THEN** the model SHALL be its own family
- **AND** `aigw model edit dmxapi/gpt-6.1-sol-cdx --family gpt-6.1-sol`, or a
  team file, SHALL link it to `gpt-6.1-sol`.

### Requirement: A model that selection adds declares no capability

A model that `aigw use` adds SHALL declare no capability, and when that turns
off Claude Desktop's tool search, the result SHALL say so and name the command
that declares the capability. `aigw model add` and `aigw model edit` SHALL
declare capabilities, and choose protocols, as `product-control-plane` defines.

#### Scenario: Claude Desktop loses tool search

- **GIVEN** every model that Claude Desktop lists declares `tool_search`
- **WHEN** `aigw use claude-desktop ucloud/claude-sonnet-5-6` adds that model
- **THEN** the result SHALL say that Claude Desktop's tool search is off
- **AND** SHALL name
  `aigw model edit ucloud/claude-sonnet-5-6 --capability tool_search`.

### Requirement: A selection commits in one guarded transaction

`aigw use` SHALL validate any Token it needs and add any model or protocol in
the same transaction as the selection, through the shared credential and
synchronization owners. A failure or cancellation before commit SHALL undo
only that attempt's writes, and a rendering failure after commit SHALL NOT undo
the committed configuration or credentials.

#### Scenario: Selection fails after Token storage

- **WHEN** writing a client configuration file or the configuration fails
- **THEN** unchanged credential postimages and a newly saved automatic backend
  choice SHALL be compensated
- **AND** any compensation error SHALL remain visible with the original failure.

#### Scenario: A selection that adds a model fails

- **WHEN** `aigw use` adds a model and then fails before commit
- **THEN** the configuration SHALL hold neither the model nor the selection.

#### Scenario: A selection needs a missing Token

- **WHEN** the selected model's provider has no Token
- **THEN** `aigw use` in a terminal SHALL prompt for it and store it only after
  the validation that `secret-storage` requires
- **AND** without a terminal it SHALL refuse, and with `--dry-run` report the
  Token as missing, without prompting, as `command-line` requires.

#### Scenario: Re-select the active model

- **WHEN** the selected model, client configuration files and required
  credential already match
- **THEN** selection SHALL succeed as an observable no-op
- **AND** SHALL NOT rewrite files, credentials or the verified snapshot.

### Requirement: Recommendations are advisory

A recommendation MAY order compatible models for a client without a selection.
Its order SHALL be declared, stable and visible. A model outside every
recommendation SHALL stay available to `aigw use` but SHALL NOT be selected
automatically, an explicit selection SHALL outrank every recommendation, and
`aigw config export` SHALL write the declared recommendations unchanged.

#### Scenario: A preferred provider is unavailable before activation

- **WHEN** a client without a selection has several recommended compatible
  models and only a lower-ranked provider's Token is available
- **THEN** setup or sync MAY select that usable model without requiring the
  higher-ranked provider
- **AND** SHALL report the choice and keep every unrelated client's selection.

#### Scenario: A provider is outside every recommendation

- **WHEN** a provider and its models are outside every recommendation for a
  client
- **THEN** AIGW SHALL keep them for explicit use
- **AND** SHALL NOT select them merely because another provider is unavailable.

#### Scenario: A selected provider fails during a request

- **WHEN** a client's request to its selected provider fails, and the operator
  then runs `aigw status` and `aigw sync` without editing a client file
- **THEN** the client's selection SHALL be unchanged
- **AND** neither command SHALL send that request again or select another
  model.

#### Scenario: An operator exports a team file

- **WHEN** the operator's clients use a recommended alternative or a model
  outside every recommendation, and the operator runs `aigw config export`
- **THEN** the team file SHALL carry the declared recommendations unchanged
- **AND** SHALL NOT contain a recommendation derived from those selections.

### Requirement: Forwarding belongs to one client

`aigw client edit <client> --via <url>` MAY send one client to a forwarding
endpoint instead of its provider's endpoint. Selection, the credential reader,
client configuration files, previews and rollback SHALL derive one destination
from that forwarding and keep the provider's Token, the model, reasoning
choices, other clients and unmanaged content. AIGW SHALL NOT manage the
forwarding service or carry requests.

#### Scenario: One client forwards to an installed endpoint

- **WHEN** an enabled client gets a forwarding endpoint through
  `aigw client edit <client> --via <url>`
- **THEN** a credential-free preview SHALL show only that client's changes
- **AND** the change SHALL keep the provider and the other clients
- **AND** the next sync SHALL keep the forwarding endpoint.

#### Scenario: A cached predecessor still requests its direct credential

- **WHEN** a running client uses its original direct credential reference
  after forwarding is set
- **THEN** the credential reader SHALL resolve that exact provider reference
- **AND** SHALL NOT copy a Token, require a new provider identity or rewrite
  conversation history.

#### Scenario: Forwarding is rolled back

- **WHEN** `aigw config rollback` restores the configuration from before
  `aigw client edit <client> --via <url>` ran
- **THEN** that client's configuration files SHALL again name its provider's
  endpoint
- **AND** its credential command SHALL return the same Token as before
  forwarding was set.

### Requirement: Forwarding follows the upstream endpoint and retry budgets follow the provider

A client's forwarding SHALL record the provider endpoint and protocol that it
reaches and SHALL be cleared, with the result saying so, whenever that upstream
endpoint changes: a change of provider, of protocol or of the provider's
endpoint. Retry budgets SHALL stay through a change of model or protocol within
the provider and SHALL be cleared, with the result saying so, on a change of
provider.

#### Scenario: A client changes model within its provider

- **WHEN** a client with forwarding and retry budgets selects another model of
  the same provider on the same protocol
- **THEN** its forwarding and retry budgets SHALL stay.

#### Scenario: A client changes protocol within its provider

- **WHEN** a client with forwarding and retry budgets selects a model of the
  same provider on another protocol
- **THEN** its forwarding SHALL be cleared and the result SHALL say so
- **AND** its retry budgets SHALL stay.

#### Scenario: A client moves to another provider

- **WHEN** `aigw use` moves a client with forwarding or retry budgets to a
  model of another provider
- **THEN** the selection SHALL drop the forwarding endpoint, its recorded
  endpoint and the retry budgets together
- **AND** the result SHALL say that they were cleared.

#### Scenario: The provider's endpoint changes

- **WHEN** `aigw provider edit <provider> --endpoint <protocol>=<url>` changes
  the endpoint that a client's forwarding records
- **THEN** that client's forwarding SHALL be cleared in the same transaction,
  and the result SHALL say so
- **AND** its retry budgets SHALL stay.

#### Scenario: The operator connects a client directly

- **WHEN** `aigw client edit <client> --direct` runs for a forwarded client
- **THEN** the client SHALL drop the forwarding endpoint and its recorded
  endpoint together.

#### Scenario: A check of another provider's model inherits no budget

- **WHEN** `aigw check <client> --model <provider/model>` names a model of
  another provider
- **THEN** the check SHALL NOT use the client's retry budgets.

### Requirement: Retry budgets belong to one client

`aigw client edit` MAY declare a client's native request and stream retry
budgets, and undeclared budgets SHALL keep the client's defaults. Preview,
resolution, client configuration files, the saved configuration and recovery
SHALL agree. AIGW SHALL NOT infer a budget from provider names or URLs,
execute retries or promise exactly-once inference.

#### Scenario: An operator delegates recovery to the endpoint

- **WHEN** `aigw client edit codex --request-retries 0 --stream-retries 0`
  runs
- **THEN** Codex's native provider table SHALL contain both zero values
- **AND** the preview and a later sync SHALL keep the declaration, provider,
  model, reasoning settings and unrelated clients.

#### Scenario: Native defaults remain undeclared

- **WHEN** a client has no retry budget, or the operator runs
  `aigw client edit <client> --native-retries`
- **THEN** AIGW SHALL omit the corresponding native provider fields
- **AND** SHALL NOT substitute a guessed default.

#### Scenario: The client cannot apply a declared budget

- **WHEN** the client has no native retry-budget setting
- **THEN** AIGW SHALL refuse before credential access, network access or
  writes
- **AND** SHALL NOT replace the setting with a helper or wrapper.

### Requirement: Retry budgets keep guarded recovery

A retry-budget change SHALL use the guarded transaction and native recovery of
selection. An edit made outside AIGW SHALL remain a client configuration
conflict, and AIGW SHALL NOT silently lose a declared budget or require a field
to be deleted by hand.

#### Scenario: A managed budget was changed outside AIGW

- **WHEN** a native budget differs from what AIGW last wrote
- **THEN** sync and selection previews SHALL report a Codex configuration
  conflict without presenting it as a TOML syntax error
- **AND** SHALL preserve the external edit and owned state.

#### Scenario: A budget change fails after it is saved

- **WHEN** writing the client configuration fails after the budgets were saved
- **THEN** compensation SHALL restore exact unchanged owned preimages
- **AND** SHALL preserve newer external edits and report incomplete
  compensation.

### Requirement: Hermes selection says existing sessions keep their model

When `aigw use` changes, or previews a change to, the client configuration of a
client whose existing sessions keep the provider and model they started with,
such as Hermes, human output SHALL say that existing sessions keep them and new
sessions use the selection. AIGW SHALL NOT change a native session's provider
or model.

#### Scenario: An operator selects another model for Hermes

- **WHEN** `aigw use hermes <provider/model>` changes the Hermes configuration
  file, or its preview would
- **THEN** human output SHALL state that existing sessions keep their provider
  and model and new sessions use the selection
- **AND** JSON output and Hermes sessions SHALL stay unchanged.

#### Scenario: The Hermes configuration stays the same

- **WHEN** a selection leaves the Hermes configuration file unchanged
- **THEN** human output SHALL NOT add the statement.

### Requirement: The client list shows each client's state

`aigw client list` SHALL list every client that AIGW supports, each with its
installed executable path or that it is not installed, whether it is enabled,
and its selected `provider/model` or that it has none.

#### Scenario: List the clients

- **GIVEN** Codex is installed, enabled and selects `dmxapi/gpt-6.1-sol`, and
  Hermes is not installed
- **WHEN** `aigw client list` runs
- **THEN** Codex's row SHALL show its executable path, that it is enabled and
  `dmxapi/gpt-6.1-sol`
- **AND** Hermes's row SHALL show that it is not installed, and Claude Code and
  Claude Desktop SHALL have rows too.

### Requirement: Disabling a client withdraws only what AIGW wrote

`aigw client disable` SHALL withdraw, through the guarded transaction that
wrote it, only that client's AIGW-owned configuration and the verified snapshot
it makes stale. `aigw self uninstall` SHALL first disable every enabled client
and remove the program only after that succeeds. Both SHALL keep the
configuration, Tokens, other clients and neighboring user state.

#### Scenario: Disable one client

- **WHEN** an operator disables one enabled client
- **THEN** only that client's AIGW-owned configuration and ownership state are
  withdrawn
- **AND** the other clients, the configuration, credentials and user-authored
  client settings remain unchanged
- **AND** the stale verified snapshot is absent.

#### Scenario: A user extends settings created by AIGW

- **GIVEN** AIGW created a previously absent client settings file
- **AND** the user later added fields outside AIGW's ownership
- **WHEN** the client is disabled or the portable program is uninstalled
- **THEN** those user fields remain and only AIGW-owned fields are withdrawn
- **AND** the file is deleted only when withdrawal leaves it empty.

#### Scenario: Uninstall the portable program

- **WHEN** an operator runs `aigw self uninstall` with one or more enabled
  clients
- **THEN** AIGW's content in every client configuration file is withdrawn
  before the executable and its rollback copy are removed
- **AND** the configuration, credentials, neighboring user state and the
  configuration backup remain available, the backup only as an explicit
  rollback source
- **AND** no verified snapshot continues to claim the withdrawn files.

### Requirement: Synchronization adopts a Hermes or Codex model choice

When Hermes's or Codex's own model selection changed to another added model,
and nothing else that AIGW owns in that client's configuration changed,
`aigw sync` SHALL record that model as the client's selection instead of
refusing, and SHALL NOT rewrite the choice. The match SHALL use the model's
exact ID and the provider and protocol of the Hermes provider entry, or those
of Codex's selection.

#### Scenario: The operator picks another listed model in Hermes

- **GIVEN** AIGW wrote several models into Hermes' catalog
- **WHEN** the operator selects another of them in Hermes and runs `aigw sync`
- **THEN** Hermes's selection SHALL name the matching `provider/model`
- **AND** Hermes SHALL keep the operator's choice
- **AND** the result SHALL report the adopted model
- **AND** other clients, credentials and Hermes sessions SHALL remain
  unchanged.

#### Scenario: The operator picks another model of the same provider in Codex

- **GIVEN** AIGW wrote a model into Codex's configuration
- **WHEN** the operator selects the ID of another added model of the same
  provider and protocol in Codex and runs `aigw sync`
- **THEN** Codex's selection SHALL name that model
- **AND** Codex SHALL keep the operator's model
- **AND** the result SHALL report the adopted model
- **AND** a following preview SHALL report no change.

#### Scenario: No added model matches the native choice

- **WHEN** no added model has the chosen ID and the provider and protocol that
  the match uses
- **THEN** synchronization SHALL fail without changing configuration or client
  files
- **AND** its Fix SHALL be `aigw use <client> <provider>/<model>` for that
  provider and the chosen ID.

#### Scenario: Other managed settings changed

- **WHEN** a managed Hermes provider entry, endpoint or credential command, or
  Codex's provider selection, provider table, scheduler, catalog or credential
  command changed outside AIGW, with or without a model change
- **THEN** synchronization SHALL refuse without changing configuration or
  client files.

#### Scenario: An explicit selection names another model

- **WHEN** the operator runs `aigw use <client> <provider/model>` for Hermes or
  Codex after choosing a different model in that client
- **THEN** AIGW SHALL write the named model, not the native choice.

### Requirement: Checking a named model leaves every selection unchanged

`aigw check <client> --model <provider/model>` SHALL check that model instead
of the client's selection and SHALL NOT change any selection, client
configuration file, Token or configuration. `--model` SHALL require exactly one
named client, a retry that the check names SHALL keep its client and model, and
`--token-stdin` SHALL require `--model` and the `endpoint` or `model` depth.
`cli-readiness` owns the depths and `secret-storage` the one-time Token.

#### Scenario: Check a named model at the endpoint depth

- **WHEN** the operator runs
  `aigw check codex --model ucloud/gpt-6.1-sol --depth endpoint`
- **THEN** only that model's provider endpoint and credential boundary SHALL be
  checked
- **AND** no selection, client configuration file, Token or configuration SHALL
  change.

#### Scenario: A named-model check lacks its scope

- **WHEN** `aigw check` gets `--model` without exactly one named client, or
  `--token-stdin` without `--model` or at the `local` or `client` depth
- **THEN** it SHALL refuse before it reads standard input or sends a request.

#### Scenario: A check of a named model names its retry

- **WHEN** `aigw check <client> --model <provider/model>` fails at the
  `client` depth, or skips a depth that only the `client` depth can prove
- **THEN** the check it names SHALL be
  `aigw check <client> --model <provider/model> --depth client`
- **AND** the same outcome without `--model` SHALL name
  `aigw check <client> --depth client`.

### Requirement: A client-depth check runs the client privately

A `client`-depth check SHALL run the real client privately, with a private
client configuration that reaches the operator's user-owned files without
copying or owning them.

#### Scenario: A client session leaves operator extensions out

- **WHEN** a `client`-depth check runs a real client
- **THEN** the client SHALL run in a private home or its native isolation mode
  without starting the operator's MCP servers, plugins, hooks, notifications or
  memory providers
- **AND** an operator extension that cannot start or authorize there SHALL NOT
  decide the result
- **AND** for Hermes the private home SHALL name no MCP servers or shell hooks
  and carry no hook consent, and the operator's configuration SHALL stay
  unchanged.

#### Scenario: The operator's Codex catalog is not an absolute path

- **WHEN** the operator's own `model_catalog_json` is relative to the Codex
  configuration that names it, or on Windows rooted without a drive, such as
  `\catalogs\models.json`
- **THEN** the private client configuration SHALL name that file by its
  absolute path, on the drive of that configuration for a rooted path
- **AND** a reference that names a drive without a root, such as
  `D:models.json`, the catalog and the operator's configuration SHALL stay
  unchanged, and AIGW SHALL NOT take ownership of the catalog.
