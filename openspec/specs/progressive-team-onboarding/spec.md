# progressive-team-onboarding Specification

## Purpose

Define progressive team setup as the independent import of a reviewed team file,
followed by explicit provider connection and configuration only of the clients
that are present on the current host.

## Requirements

### Requirement: Setup has one explicit commit boundary

Setup SHALL own configuration, credential slots, backend selection, client
configuration files, verified snapshots, locks, and temporary files as one
transaction until persistence and every participating client configuration file
succeed. A configuration write SHALL NOT establish completion. Failure SHALL
compensate only unchanged postimages, preserve concurrent user changes, and
report the original cause plus unresolved resources. Presentation SHALL remain
outside the transaction.

#### Scenario: Setup fails before commit

- **WHEN** guided or team-file setup fails before the configuration and every
  selected client's configuration file are written, including after the
  configuration file is written
- **THEN** compensation restores unchanged owned preimages and removes that
  attempt's new resources wherever recovery succeeds
- **AND** any failed recovery or concurrent postimage conflict remains explicit
  without overwriting newer user state or claiming complete restoration.

#### Scenario: Setup output fails after commit

- **WHEN** setup commits configuration, credentials, backend selection, and
  client configuration files but result rendering fails
- **THEN** the committed product state remains available
- **AND** the command reports only the output failure rather than claiming that
  setup was rolled back.

### Requirement: The shipped team file keeps one general family per other vendor

The shipped team file SHALL hold one family for each product line of Anthropic, OpenAI and Google, and exactly one
general family for each other vendor. A curated line's family SHALL be its newest generally available model, and
another vendor's family its most capable generally available general model, whose provider models qualify. A
qualified successor SHALL replace its family and models, and applying the file SHALL keep explicit client selections
of the replaced models.

#### Scenario: A vendor's more capable general model becomes available

- **GIVEN** the team file holds one general family for a vendor
- **WHEN** that vendor makes a more capable general model generally available
  and its models qualify
- **THEN** the team file SHALL replace that vendor's family and models with the
  successor
- **AND** SHALL NOT keep both as general families
- **AND** applying the team file SHALL keep explicit client selections of the
  replaced models.

#### Scenario: A newer model is not generally available or not qualified

- **WHEN** a vendor's newer model is limited to selected testers or its models
  fail qualification
- **THEN** the team file SHALL keep that vendor's current family.

#### Scenario: The owner names a curated vendor's families

- **WHEN** the owner names the product lines of Anthropic, OpenAI or Google,
  such as Claude Fable, Opus, Sonnet and Haiku
- **THEN** the team file SHALL hold one family for each named line, with every
  provider model of it that qualifies, channel variants included
- **AND** a line that the owner no longer names SHALL leave the file.

#### Scenario: A curated line makes a newer model generally available

- **WHEN** a curated line makes a newer model generally available and its
  provider models qualify, such as GPT-6.2 Astra after GPT-6 Astra
- **THEN** the team file SHALL replace that line's family and models with the
  newer model
- **AND** the repository's tests SHALL fail while one line holds two families.

### Requirement: Guided setup reaches a working state and can run again

`aigw setup` without `--from` SHALL guide a person through adding or choosing a
provider, choosing an installed client and choosing a model from that provider's
live catalog, or typing a model ID when it lists none, and SHALL then check that
client as `aigw check <client>` does. It SHALL run again on an existing
configuration and change only what the person chooses. Without a terminal, or
with `--json` or `--dry-run`, it SHALL refuse and name
`aigw setup --from <file>`.

#### Scenario: Guided setup runs on a new machine

- **WHEN** `aigw setup` runs in a terminal without configuration
- **THEN** it SHALL ask for a provider's ID, endpoints and Token and validate
  the Token as `secret-storage` defines
- **AND** it SHALL then ask for an installed client and a model from that
  provider's catalog, save them in one transaction and check that client.

#### Scenario: The catalog lists no model

- **WHEN** the provider's catalog cannot be read or lists no model
- **THEN** guided setup SHALL accept a typed model ID instead of a choice from
  the catalog.

#### Scenario: Guided setup runs again

- **WHEN** `aigw setup` runs in a terminal on a configuration that already has
  providers and client selections
- **THEN** it SHALL start instead of refusing and offer the existing providers
  beside adding a new one
- **AND** the providers, Tokens and client selections that the person does not
  change SHALL stay unchanged.

#### Scenario: Guided setup cannot run

- **WHEN** `aigw setup` runs without `--from`, and without a terminal or with
  `--json` or `--dry-run`
- **THEN** it SHALL refuse without prompting, reading a Token or writing a file
- **AND** SHALL name `aigw setup --from <file>`.

#### Scenario: Guided setup offers installed clients

- **WHEN** guided setup asks for a client
- **THEN** it SHALL offer only the supported clients that are installed
- **AND** it SHALL require a protocol choice when the client and the model share
  more than one.

#### Scenario: No client is installed

- **WHEN** guided setup finds no installed supported client
- **THEN** it SHALL add the provider and stop before asking for a model
- **AND** it SHALL name `aigw client list`.

#### Scenario: A Token already exists for the new provider

- **GIVEN** the credential store already holds a Token for a provider ID that
  the configuration does not name
- **WHEN** guided setup adds that provider
- **THEN** the stored Token SHALL NOT by itself count as a configured provider
- **AND** setup, with or without `--from`, SHALL use it without a prompt and
  say so, unless the operator replaces it explicitly with guarded
  compensation.

#### Scenario: Guided setup enables the client it configures

- **GIVEN** a client that is disabled on a model of its own
- **WHEN** guided setup selects that client and another model for it
- **THEN** the client SHALL be enabled
- **AND** the saved rows SHALL say that it was disabled and is now enabled.

### Requirement: A team file merges without redirecting providers or clients

`aigw setup --from <file>` SHALL apply a team file in one transaction. The
models, families and recommendations that it names SHALL replace the local ones,
and local-only entries SHALL stay unless `--prune` removes the unused ones. It
SHALL add new providers, take the file's provider labels and fill in a missing
balance probe, but SHALL NOT change an existing provider's endpoints or Token or
replace a client's selection; a differing endpoint or balance probe SHALL stay
local and be reported.

#### Scenario: The file and the configuration define the same model

- **WHEN** the team file and `aigw.toml` both define `dmxapi/gpt-6.1-sol` with
  different labels, families, protocols or capabilities
- **THEN** the model SHALL take the team file's definition
- **AND** models, families and recommendations that only `aigw.toml` names
  SHALL stay.

#### Scenario: The file names an existing provider with another label

- **WHEN** the team file gives `dmxapi` a label that differs from the local one
- **THEN** the provider SHALL take the file's label.

#### Scenario: The file names another endpoint or balance probe

- **WHEN** the team file gives `dmxapi` a `responses` endpoint or a balance
  probe that differs from the local one
- **THEN** the local value and the provider's Token SHALL stay
- **AND** setup SHALL report both values.

#### Scenario: The file gives a balance probe to a provider without one

- **WHEN** the team file gives `ucloud` a balance probe and the local `ucloud`
  has none
- **THEN** the provider SHALL take the file's balance probe.

#### Scenario: A client already has a selection

- **WHEN** the team file recommends another model for a client that already
  selects one
- **THEN** the client's selection SHALL stay
- **AND** the file's recommendation SHALL be stored for that client.

#### Scenario: The file would take a protocol from a client's selection

- **WHEN** the team file's definition of a model omits the protocol of a
  client's selection of that model
- **THEN** the model SHALL keep its local definition
- **AND** setup SHALL report that it kept it.

#### Scenario: A kept provider has no endpoint for a model's protocol

- **WHEN** the team file defines a model of an existing provider on a protocol
  for which the kept provider has no endpoint
- **THEN** setup SHALL add the model and report it as unusable on that protocol
  until `aigw provider edit --endpoint` supplies the endpoint
- **AND** no client SHALL be given the model on that protocol until then.

#### Scenario: The file's capabilities were qualified on another endpoint

- **WHEN** the team file declares capabilities on a protocol whose endpoint in
  the file differs from what the kept provider has
- **THEN** setup SHALL drop those capabilities and report them
- **AND** the model SHALL keep the capabilities that it has locally on that
  protocol.

#### Scenario: A recommendation names a model

- **WHEN** a recommendation names `dmxapi/gpt-6.1-sol-cdx`
- **THEN** setup SHALL store it under that `provider/model`
- **AND** a name that neither the team file nor `aigw.toml` defines SHALL be
  refused before any write, naming that recommendation.

#### Scenario: Prune unused local entries

- **WHEN** `aigw setup --from <file> --prune` runs
- **THEN** the local-only models that no client selects and no remaining
  recommendation names SHALL be removed in the same transaction as the merge
- **AND** so SHALL families that neither a remaining model nor the file names
- **AND** providers, their endpoints and Tokens, and every client's selection
  SHALL stay unchanged.

#### Scenario: The file lists providers or models in another order

- **WHEN** a team file lists providers, or one provider's models, in another
  order than the configuration
- **THEN** what the file names SHALL come first, in the file's order, and the
  entries that only the configuration holds SHALL follow in their local order,
  for providers and for each provider's models alike
- **AND** the merge SHALL change only that display order, never an endpoint, a
  Token or a client's selection.

#### Scenario: Preview a team file

- **WHEN** `aigw setup --from <file> --dry-run --json` runs
- **THEN** it SHALL apply the same merge validation as the write and report
  each difference that stays local, each local model whose definition the file
  replaces, every label that the file replaces on a provider, a model or a
  family, with its local and its new value, each model that it would keep or
  report as unusable, each capability that it would drop, and the client
  selections that it would fill
- **AND** the write SHALL report the same labels, and a preview after it SHALL
  report none
- **AND** it SHALL NOT describe a client configuration file as written or as
  proof that the client or its Token works.

### Requirement: Setup refuses a team file in another form

`aigw setup --from` SHALL refuse, before any write, a team file that has a
`[clients]` section or a `version` other than 8. The refusal for version 7 SHALL
name that file's conversion path.

#### Scenario: A team file has a clients section

- **WHEN** a team file contains a `[clients]` section
- **THEN** setup SHALL refuse before any write.

#### Scenario: A team file has another version

- **WHEN** a team file's `version` is not 8
- **THEN** setup SHALL refuse before any write and name that version.

### Requirement: Team setup connects Tokens without requiring them

`aigw setup --from` SHALL require no particular provider's Token, client or
external service. It SHALL prompt for a missing Token only in a terminal, take
one provider's Token from standard input when `--token-stdin` comes with
`--provider <id>`, and otherwise defer that provider and name its fix. A
deferred Token SHALL NOT fail setup. A Token that it stores SHALL first pass the
validation that `secret-storage` defines.

#### Scenario: No Token or client is present

- **WHEN** a team file is applied without a terminal on a host with no
  supported client and no provider Token
- **THEN** setup SHALL save the file's providers, models, families and
  recommendations without prompting
- **AND** SHALL report that no provider has a Token
- **AND** SHALL show the smallest provider-scoped command that supplies one.

#### Scenario: A script supplies one provider's Token

- **WHEN** `aigw setup --from team.toml --provider dmxapi --token-stdin` runs
- **THEN** only the `dmxapi` Token SHALL be read from standard input, validated
  and stored
- **AND** each other provider without a Token SHALL be deferred with its fix.

#### Scenario: The provider does not accept a supplied Token

- **WHEN** a prompted or piped Token fails that validation
- **THEN** setup SHALL refuse, and neither that Token nor the team file's
  changes SHALL be stored.

#### Scenario: Token input names more than one provider

- **WHEN** `--token-stdin` comes with more than one `--provider`
- **THEN** setup SHALL refuse before reading the file or standard input.

#### Scenario: A named provider's Token is requested in a terminal

- **WHEN** `aigw setup --from <file> --provider dmxapi` runs in a terminal
- **THEN** setup SHALL prompt only for the `dmxapi` Token, without echoing it.

#### Scenario: A provider is named without a terminal or Token input

- **WHEN** `aigw setup --from <file> --provider dmxapi` runs without a terminal
  or with `--json`, and without `--token-stdin`
- **THEN** setup SHALL refuse before any write and name `--token-stdin`.

#### Scenario: A terminal asks for a needed Token

- **WHEN** `aigw setup --from <file>` runs in a terminal without `--provider`,
  and a provider whose recommended model setup would select has no Token
- **THEN** setup SHALL prompt for that Token, and for no other provider's,
  without echoing it
- **AND** a `--json` or non-terminal run SHALL defer that provider, because
  `command-line` lets neither prompt.

#### Scenario: Tokens come from the environment

- **WHEN** `--token-stdin` is given while the read-only environment backend
  supplies Tokens
- **THEN** setup SHALL refuse before any write and name the provider's
  environment variable.

#### Scenario: One provider's Token is available

- **WHEN** exactly one provider in the team file has a Token
- **THEN** setup SHALL fill each client without a selection from that
  provider's recommended models, in declared order
- **AND** a model that no recommendation names SHALL stay manual-only
- **AND** the other providers' missing Tokens SHALL NOT block setup.

### Requirement: Team setup configures only present clients

`aigw setup --from` SHALL write client configuration files only for installed
clients whose selected model's provider has its Token, without running a client,
and without contacting a provider except to validate a Token that it stores. A
stored Token and a written client configuration file SHALL NOT be presented as
accepted authentication, a reachable endpoint or working inference; `aigw check`
owns those observations.

#### Scenario: Only Claude Code is installed

- **WHEN** a provider with a Token has models recommended for Claude Code and
  for Codex, and only Claude Code is installed
- **THEN** setup SHALL write only Claude Code's configuration file
- **AND** an unavailable `responses` endpoint SHALL NOT block setup.

#### Scenario: Provider is offline during team setup

- **WHEN** one provider already has a stored Token and cannot be reached
- **THEN** setup SHALL keep the merged configuration, the selections and the
  client configuration files without an online request
- **AND** SHALL NOT report endpoint or inference checks as passed
- **AND** a later `aigw check` SHALL report the provider failure without
  exposing the Token.

#### Scenario: A client is installed later

- **WHEN** a team file was applied before a supported client was installed
- **AND** `aigw sync` runs after that client is installed
- **THEN** AIGW SHALL discover the client and write its configuration file
- **AND** SHALL preserve unrelated client and conversation state.

### Requirement: Onboarding results name deferred work

Setup results SHALL distinguish what the team file changed, which providers
have Tokens, which clients are selected and configured, and deferred work, and
SHALL name the smallest safe next action. With environment Tokens, setup SHALL
list every compatible variable as an equal choice. Setup SHALL offer `--json`
with the same result and SHALL never expose a credential.

#### Scenario: A writable store has no Token

- **WHEN** a team file is applied and no provider has a Token in a writable
  credential store
- **THEN** setup SHALL name `aigw provider rotate <provider>`
- **AND** SHALL NOT imply that every provider's Token is required.

#### Scenario: A Token arrives after setup

- **GIVEN** a team file was applied and no client has a selection
- **WHEN** the operator rotates one compatible provider's Token
- **THEN** rotation SHALL leave every client's selection unchanged
- **AND** SHALL name `aigw use <client> <provider/model>` for one recommended
  compatible model of that provider, rather than `aigw check` or a sync that
  changes nothing.

#### Scenario: The environment backend has no Token

- **WHEN** a team file is applied with the read-only environment backend and
  no provider Token is available
- **THEN** setup SHALL list the environment variable of every compatible
  provider as an alternative
- **AND** SHALL direct the operator to run `aigw sync` after setting one
- **AND** SHALL state that any one compatible provider is sufficient.

#### Scenario: Nothing can be activated yet

- **WHEN** a team file is applied with no compatible Token and no supported
  client
- **THEN** setup SHALL report the file as applied without claiming that any
  endpoint, credential or client is ready
- **AND** setup, `status`, `check` and `sync --dry-run` SHALL name the same
  prerequisite: any one compatible provider Token
- **AND** `sync --dry-run` SHALL select no model by recommendation alone.

#### Scenario: A client is not installed yet

- **WHEN** a provider has its Token and an enabled client's selection has no
  client configuration file because the client is not installed
- **THEN** the provider, Token, selection and enabled intent SHALL stay
  distinct from the missing client and the unchecked inference
- **AND** setup, `use`, `sync --dry-run`, `status` and `check` SHALL name
  installing the client followed by `aigw sync`, not `aigw check`
- **AND** `check` SHALL NOT probe that client's endpoint before its
  configuration file exists.

#### Scenario: A selected Token disappears before client installation

- **WHEN** an enabled client has no configuration file yet and the Token of
  its selected provider becomes unavailable
- **THEN** read-only commands SHALL report the missing Token and the missing
  client as distinct facts without reading the Token value
- **AND** the next action SHALL restore that provider's Token before
  suggesting `aigw sync` or a check.

#### Scenario: Enabled clients have different local readiness

- **WHEN** a provider has its Token, Codex's configuration file is written,
  and enabled Claude Code is not installed yet
- **THEN** `sync --dry-run`, `status` and `check` SHALL keep both selections
  and give the same installation-then-sync next step in human and JSON output
- **AND** `check` MAY probe Codex's selected model but SHALL NOT probe Claude
  Code's or claim that every enabled client passed.

#### Scenario: Automation reads the setup result

- **WHEN** `aigw setup --from <file> --json` runs
- **THEN** its document SHALL carry the providers, models and families that
  the file added, the providers with Tokens, each client's state, the
  alternative Token choices and the next action
- **AND** SHALL NOT include a Token or another credential value.

#### Scenario: A selected responses endpoint is unavailable

- **WHEN** an installed Codex selects a model whose `responses` endpoint is not
  reachable
- **THEN** `aigw check` SHALL identify that configured endpoint as unavailable
- **AND** SHALL state that AIGW owns configuration rather than the endpoint's
  lifecycle
- **AND** SHALL offer checking that endpoint or selecting another `responses`
  model, without inferring the endpoint's implementation.

### Requirement: Deferred activation resumes through sync

`aigw sync` SHALL later configure newly installed clients and newly available
environment Tokens without repeating setup or selecting in bulk. Setup and sync
SHALL fill only clients without a selection, from the declared primary and
then the alternatives in order, and only with a usable Token. A model that no
recommendation names SHALL stay manual-only, and an explicit selection SHALL
never be replaced.

#### Scenario: An environment Token becomes available later

- **WHEN** an environment Token for a provider becomes available after a team
  file was applied
- **THEN** `aigw sync` SHALL select that provider's first compatible
  recommended model, with its declared protocol, for each client without a
  selection
- **AND** SHALL NOT require the primary provider or any unrelated Token
- **AND** existing client selections SHALL stay.

#### Scenario: A recommendation survives deferred setup

- **WHEN** setup stores a recommendation while no required Token is available
- **THEN** the recommendation SHALL persist without becoming a selection
- **AND** later activation SHALL follow the declared order, not alphabetical
  order.

#### Scenario: An explicit selection is temporarily unavailable

- **WHEN** a client selects a model whose provider has no Token and another
  compatible provider gains one
- **THEN** setup and `aigw sync` SHALL keep that selection
- **AND** SHALL NOT replace it with a recommendation or another provider.

#### Scenario: A recommended model is removed

- **WHEN** an unselected recommended model is removed
- **THEN** its recommendation SHALL change in the same configuration
  transaction
- **AND** other recommendations and every client's selection SHALL stay.

#### Scenario: Codex is installed after setup

- **WHEN** Codex is installed after a team file was applied
- **THEN** `aigw sync` SHALL write only AIGW's values into Codex's
  configuration
- **AND** `aigw status` SHALL name explicit authentication as the next step
  when Codex needs it
- **AND** no unrelated provider Token SHALL be required.

### Requirement: The shipped team file is reviewed and directly usable

The repository SHALL publish one token-free, reviewed `manifests/team.toml` in
schema 8 that `aigw setup --from` applies without credentials or installed
clients. It SHALL declare providers, models, families and recommendations, with
stable lower-case family IDs and each model ID exactly as its provider serves
it. A model SHALL enter it only with authenticated inference and client and
protocol evidence, and it SHALL hold no inferred model matrix or proxy endpoint.

#### Scenario: The shipped team file stays reviewed

- **WHEN** the repository ships its team file
- **THEN** fictitious providers, workstation paths and parallel example files
  SHALL NOT remain
- **AND** a model outside the recommendations SHALL require explicit selection
  even if its provider has a Token or its family matches.

#### Scenario: A team member applies the reviewed file

- **WHEN** a team member downloads `manifests/team.toml` and applies it with
  `aigw setup --from`
- **THEN** AIGW SHALL apply its providers, models, families and
  recommendations without a second provider or model-name policy
- **AND** Tokens SHALL stay outside the file
- **AND** recommended selections SHALL come from the file rather than from
  model-version literals in this specification.

#### Scenario: One provider offers a subset of families

- **WHEN** one provider with a Token has a compatible recommended model but no
  models for other families or clients
- **THEN** setup SHALL fill only the clients without a selection that it can
  serve
- **AND** SHALL keep explicit selections and defer the other capabilities.

#### Scenario: A provider uses a channel-specific model ID

- **WHEN** DMXAPI serves a CC, SSVIP or CDX channel variant of one vendor model
- **THEN** that model SHALL name the same family as its base model and keep the
  provider's exact model ID.

#### Scenario: A model needs no duplicate label

- **WHEN** a model omits `label`
- **THEN** human, JSON and client presentation SHALL derive its label from the
  provider's and the family's labels, as `Provider · Family`, or as
  `Provider · model-id` when the model is its own family
- **AND** an explicit channel or user label SHALL override that derivation
- **AND** `aigw config export` SHALL omit a stored label equal to the derived
  one while keeping a distinct explicit label.

### Requirement: Configuration export writes a team file

`aigw config export` SHALL write the configuration as a schema 8 team file: its
providers without Tokens or balance credentials, models, families and
recommendations, in configuration order, without a `[clients]` section, and
`aigw setup --from` SHALL accept the result.

#### Scenario: Export a configuration

- **WHEN** `aigw config export` runs on a configuration with providers, models,
  families, recommendations and client selections
- **THEN** its output SHALL hold the providers, models, families and
  recommendations in configuration order
- **AND** SHALL hold no `[clients]` section, Token or balance credential.

#### Scenario: Apply an exported team file

- **WHEN** another machine runs `aigw setup --from` with the exported file
- **THEN** setup SHALL accept it and apply its providers, models, families and
  recommendations.

### Requirement: Adding a provider is one guarded transaction

`aigw provider add <provider>` SHALL create a provider with its label and
endpoints in the same guarded transaction that stores its Token, validated as
`secret-storage` defines, and SHALL select no model for any client. An existing
provider ID SHALL be refused before a Token is requested, and a failed save
SHALL compensate the stored Token through the shared replacement owner.

#### Scenario: A script adds a provider

- **WHEN** a script runs `aigw provider add` with `--endpoint` and
  `--token-stdin`, and the provider accepts the Token
- **THEN** the provider and its Token SHALL be stored
- **AND** no client's selection or configuration file SHALL change.

#### Scenario: The provider already exists

- **WHEN** `aigw provider add` names an existing provider ID
- **THEN** it SHALL refuse before requesting a Token or changing anything.

#### Scenario: Saving fails after the Token is stored

- **WHEN** the Token is stored but saving the configuration fails
- **THEN** the shared Token replacement owner SHALL compensate the Token
- **AND** the original failure and any compensation error SHALL be preserved
- **AND** failed compensation SHALL NOT be reported as restored storage.
