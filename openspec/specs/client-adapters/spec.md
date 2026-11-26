# client-adapters Specification

## Purpose

Define how an admitted client's adapter joins AIGW: its admission, the extension
boundary that every adapter shares, the client surfaces it serves, its native
provider declarations and how the control plane converges each client.

## Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independent Adapters owning discovery,
projection, authentication, rollback, verification, status, and withdrawal of
AIGW state. Provider-Token credential commands MUST use an absolute qualified
reader, never store Tokens in client files, and never rely on a package-manager
CLI link. Client-native authentication and
preferences SHALL remain client-owned. A new client MUST add an Adapter without
changing provider policy or other Adapters.

#### Scenario: Verification uses synchronized settings

- **WHEN** AIGW verifies an admitted client
- **THEN** it SHALL use the client's synchronized settings without a wrapper.

#### Scenario: One admitted client is absent

- **WHEN** setup discovers only Codex or only Claude Code
- **THEN** AIGW SHALL configure only the present client
- **AND** it SHALL explicitly leave the absent client untouched

#### Scenario: Claude launches outside the installer shell

- **WHEN** Claude Code requests a credential from an enabled AIGW projection
- **THEN** `apiKeyHelper` SHALL invoke the selected qualified reader
- **AND** credential retrieval SHALL not depend on the caller's PATH
- **AND** the projected settings SHALL contain no plaintext Token

#### Scenario: Codex authenticates an explicit native provider

- **WHEN** Codex's enabled selection names an explicit native provider identity
- **THEN** its declared authentication mode SHALL determine credential ownership
- **AND** provider-Token authentication SHALL invoke the selected qualified
  reader with the fingerprint of Codex's selected provider and endpoint
- **AND** client-native authentication SHALL project no AIGW Token helper
- **AND** neither mode SHALL write a plaintext Token to public configuration

#### Scenario: Claude uses an Anthropic-compatible provider

- **WHEN** explicit verification invokes Claude Code for a selected model
- **THEN** the native client SHALL consume its synchronized settings in bare,
  nonpersistent mode
- **AND** stale AIGW-owned Anthropic environment overrides SHALL be removed
- **AND** unrelated client preferences, including beta controls, SHALL remain
  unchanged rather than becoming a new AIGW authority

#### Scenario: The installed executable path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the AIGW executable path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: The credential reader path is invalid

- **WHEN** a client credential projection is prepared with a relative path or
  control character in the selected reader path
- **THEN** the transaction SHALL fail before writing the owned projection
- **AND** existing user-owned settings SHALL remain unchanged

#### Scenario: Credential retrieval is not admitted

- **WHEN** a credential request names an unsupported client, a disabled
  client, a fingerprint that matches no provider endpoint, or a provider
  without a Token
- **THEN** the request SHALL fail without writing credential bytes to standard
  output

#### Scenario: A future agent is admitted

- **WHEN** Hermes or another agent supporting third-party LLM APIs is proposed
- **THEN** admission SHALL require only that agent's adapter, declaration, and
  fixtures and SHALL NOT change provider policy, external-gateway behavior,
  command roots, or an existing adapter

#### Scenario: Codex CLI and Desktop share one home

- **WHEN** Codex uses the same configuration home for CLI and Desktop
- **THEN** AIGW SHALL project the selected model once into that shared home
- **AND** SHALL NOT create a second Desktop-specific configuration authority

#### Scenario: An adapter is proposed for another agent client

- **WHEN** a new agent client such as pi or OpenCode is proposed
- **THEN** its admission SHALL independently prove executable discovery,
  protocol/model selection, credential ownership, native configuration merge,
  rollback, withdrawal and a real tool-loop journey
- **AND** AIGW SHALL NOT advertise support based only on a writable config
  path or a successful endpoint probe.

### Requirement: Client-scoped Codex native provider

The `[clients.codex]` table SHALL remain the sole owner of its optional native
`model_provider`. A missing `model_provider` SHALL resolve to the canonical
`aigw` provider. It SHALL NOT be inferred from a provider, endpoint, proxy
implementation, or client-private state. Authentication ownership SHALL be
independent of the `model_provider` name.

#### Scenario: Explicit Codex provider

- **WHEN** `[clients.codex]` declares a safe `model_provider`
- **THEN** its resolved runtime carries that exact provider identity
- **AND** Codex receives one attributed provider table using the endpoint of
  the selected model's provider and the declared authentication mode.

#### Scenario: Default Codex provider

- **WHEN** `[clients.codex]` omits `model_provider`
- **THEN** its runtime resolves the canonical `aigw` provider
- **AND** provider-Token authentication uses the same command-helper contract
  as an explicit provider.

#### Scenario: Provider ownership is narrow

- **WHEN** another client's table or an unsafe provider identifier declares
  `model_provider`
- **THEN** configuration validation fails before persistence or projection.

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic.
Endpoints and models SHALL enter as provider data; existing client-native
authentication SHALL be reused through explicit selections. New clients SHALL
require complete Adapters, unsupported credential contracts SHALL require
admission, and incompatible wire behavior SHALL remain in a separate data
plane. Gateways SHALL remain optional endpoints. Dependencies MAY be admitted
only when total owned complexity falls.

#### Scenario: Add a compatible Provider endpoint

- **WHEN** an endpoint satisfies an admitted client protocol and the existing
  provider authentication contract
- **THEN** an operator SHALL add it through configuration data
- **AND** AIGW SHALL NOT add provider-name branching or a provider-specific
  runtime package.

#### Scenario: Add a client integration

- **WHEN** a new local client requires AIGW-managed configuration
- **THEN** it SHALL be admitted through its own discovery, planning, guarded
  projection, verification, rollback, and uninstall boundary
- **AND** it SHALL NOT reuse another client's conditional path or private state.

#### Scenario: A client adapter is registered

- **WHEN** an adapter is registered, including a test-only rehearsal adapter
- **THEN** it SHALL pass one shared adapter contract: its plan lists every
  file that applying it may change and states exactly whether applying it
  changes one, rollback restores the prior bytes and modes, withdrawal leaves
  no owned file, and inspection, verification and planning change no file and
  leave the caller's configuration unchanged
- **AND** an adapter that fails any part of that contract SHALL NOT be admitted.

#### Scenario: Compose with a traffic gateway

- **WHEN** an operator selects a general gateway or narrow compatibility
  service
- **THEN** AIGW SHALL model its URL as an ordinary provider endpoint
- **AND** SHALL NOT install, supervise, configure, embed, or copy the service's
  traffic policy.

#### Scenario: Evaluate a mature dependency

- **WHEN** a library or framework is proposed for an AIGW-owned boundary
- **THEN** admission SHALL demonstrate a net reduction in owned complexity
- **AND** popularity or feature count alone SHALL NOT justify adoption.

### Requirement: Extension preserves the control-plane core

An ordinary provider SHALL be added through provider, endpoint, model, family,
and optional balance-probe declarations. A new client SHALL be added through
one admitted client Adapter and its conformance fixtures. Neither extension
SHALL introduce provider-name branching in the core, reuse another client's
projection, or make an optional product a dependency.

#### Scenario: Add a client-native cloud model service

- **WHEN** an AWS model service exposes an admitted client protocol and the
  client already owns its credential chain and request signing
- **THEN** its provider and models use the ordinary team-file path
- **AND** that client's `[clients.<id>]` table declares client-native
  authentication
- **AND** AIGW neither stores the cloud credential nor implements a signing
  Adapter.

#### Scenario: Add another agent client

- **WHEN** Hermes, OpenCode, Pi, Qoder, or another client is admitted
- **THEN** it supplies its own discovery, projection, credential, rollback,
  verification, and uninstall contract
- **AND** existing provider data and client Adapters remain unchanged.

#### Scenario: Supply operational adapters in a different order

- **WHEN** the registry receives operational adapters in an order different
  from its admitted client declarations
- **THEN** discovery, planning and all-client application SHALL follow admission
  order, independently of constructor argument order
- **AND** a failed application SHALL compensate completed adapters in reverse
  application order.

#### Scenario: An adapter's compensation fails

- **WHEN** application fails after prior adapters have completed and one prior
  adapter reports a compensation failure
- **THEN** the registry SHALL still attempt every other prior adapter's
  compensation in reverse application order
- **AND** the returned error SHALL preserve the application failure and each
  compensation failure without claiming successful restoration.

### Requirement: Client convergence is scoped and monotonic

AIGW SHALL derive each client's state from that client's entry and its
selected model, provider, family, Adapter, and credential backend. Only the
client's entry owns its selection and enabled intent. Setup, selection, sync,
and readiness SHALL require no global selection, other client's entry, or
external compatibility product. Discovery SHALL not create native state or
enable a client. Projection SHALL preserve unrelated fields and reject changed
preimages.

#### Scenario: A client is discovered but not configured

- **WHEN** discovery finds a supported executable whose native state has not
  been initialized or given a selection
- **THEN** AIGW SHALL report it as available without creating its configuration
- **AND** synchronization and readiness of other clients remain unaffected.

#### Scenario: Both clients are selected independently

- **WHEN** an operator selects one model for Codex and one for Claude Code in
  separate operations
- **THEN** both clients' selections remain
- **AND** readiness requires no additional aggregate selection operation.

#### Scenario: An ordinary configuration edit affects one client

- **WHEN** a provider, model, or family edit changes one client's configuration
  files
- **THEN** the configuration transaction SHALL plan and apply only the affected
  client set in admission order
- **AND** another client's external edits and ownership state SHALL remain
  byte-identical, without being inspected as a prerequisite for that edit.

#### Scenario: A shared edit affects multiple clients

- **WHEN** an edit changes the configuration files of several clients
- **THEN** all affected clients SHALL participate in the same guarded
  transaction and existing reverse compensation contract
- **AND** explicit synchronization SHALL still reconcile its entire requested
  client scope, including unchanged configuration.

#### Scenario: One client is absent

- **WHEN** a valid selection belongs to a client that is not installed
- **THEN** AIGW records that capability as deferred
- **AND** the installed client's independent selection remains usable.

#### Scenario: An external compatibility endpoint is absent

- **WHEN** no local compatibility service is installed
- **THEN** AIGW remains usable with any configured native HTTPS endpoint
- **AND** no external-gateway file, process, service, port, or lifecycle state
  is required.

#### Scenario: Cancellation precedes mutation admission

- **WHEN** setup, selection, projection, or synchronization observes a
  cancelled request at its mutation boundary
- **THEN** it returns cancellation without starting that boundary's writes
- **AND** setup restores any Token already prepared before cancellation was
  observed, subject to the existing credential ownership guard.

#### Scenario: Preparation observes cancellation before persistence

- **WHEN** snapshot capture or projection preflight completes with the request
  cancelled
- **THEN** configuration persistence does not start
- **AND** existing configuration, backup, verified snapshot and client files
  remain unchanged.

#### Scenario: Cancellation prevents a later client projection

- **WHEN** cancellation is observed before the next client adapter starts
- **THEN** that adapter performs no writes
- **AND** the registry compensates prior adapters in reverse order and the
  enclosing transaction compensates its configuration
- **AND** cancellation and any recovery conflicts remain inspectable errors.

#### Scenario: Cancellation follows the last admitted projection

- **WHEN** the last admitted adapter completes successfully after cancellation
  arrives during its synchronous operation
- **THEN** the completed transaction remains successful
- **AND** no post-completion cancellation check undoes its committed result.

### Requirement: Hermes and Claude Desktop have explicit adapters

AIGW SHALL integrate Hermes and Claude Desktop through their supported native
configuration and credential interfaces. Each Adapter SHALL use the shared
selection, transaction, discovery, verification, and withdrawal owners. Claude
Desktop Chat, Cowork, and Code SHALL retain distinct capability evidence.
Hermes provider IDs SHALL join `aigw`, the provider ID and the protocol name.
Client-owned sessions, services, and model choices SHALL remain client-owned.

#### Scenario: Hermes becomes available after team setup

- **GIVEN** a team file was applied before Hermes was installed
- **WHEN** Hermes becomes available and a model is explicitly selected for it
- **THEN** synchronization SHALL configure its provider and model
- **AND** credentials SHALL use the selected supported delivery mechanism
- **AND** unrelated clients, Hermes sessions, and Hermes services remain
  unchanged.

#### Scenario: Hermes exposes the connected catalog

- **GIVEN** more than one provider has an available Token
- **WHEN** Hermes synchronization projects its native model catalog
- **THEN** every model compatible with Hermes from those providers SHALL be
  grouped under its provider and protocol
- **AND** the explicitly selected model SHALL remain Hermes' active model
- **AND** providers without a Token and models that Hermes cannot use SHALL
  remain absent.

#### Scenario: Claude Desktop lists one entry per family

- **WHEN** AIGW projects the Claude Desktop profile
- **THEN** it SHALL list the selected model and, for each other family of the
  selected provider and protocol, the first such model in configuration order
- **AND** a model that is its own family SHALL count as one family.

#### Scenario: Hermes provider IDs name the protocol

- **WHEN** AIGW projects Hermes provider entries for provider `dmxapi`
- **THEN** their IDs SHALL be `aigw-dmxapi-anthropic`, `aigw-dmxapi-responses`
  and `aigw-dmxapi-chat` for the `anthropic`, `responses` and `chat`
  protocols.

#### Scenario: Claude Desktop and Claude Code select different models

- **WHEN** the operator selects different models for Claude Desktop and Claude
  Code
- **THEN** each surface SHALL use its own documented configuration boundary
- **AND** status SHALL state which Desktop modes and platforms were qualified
- **AND** a required app restart SHALL be reported before activation is claimed.

#### Scenario: Claude Desktop is checked at depth client

- **WHEN** `aigw check --depth client` covers Claude Desktop
- **THEN** its adapter SHALL start no Claude Desktop session, and the depth
  SHALL be skipped with the reason that its acceptance happens inside the app
- **AND** Claude Desktop's row SHALL name the deepest depth that it reached.

### Requirement: Claude Desktop tool search follows model capability

AIGW SHALL enable Claude Desktop host tool search in its owned gateway profile
only when every model that the profile lists declares the `tool_search`
capability on `anthropic`. That capability records a native Desktop session's
qualification of the tool-search beta, deferred tool loading and
`tool_reference` results on that protocol. Configuration validation SHALL
reject `tool_search` on any other protocol.

#### Scenario: Every listed model declares tool search

- **WHEN** the selected model and every other model in the Desktop profile
  declare `tool_search` on `anthropic`
- **THEN** synchronization SHALL write the profile with tool search enabled
- **AND** inspection SHALL report that client configuration file unchanged.

#### Scenario: One listed model lacks tool search

- **WHEN** any model in the Desktop profile does not declare `tool_search`
- **THEN** the profile SHALL omit the tool-search setting
- **AND** a tool-search setting added outside AIGW SHALL fail closed as
  managed drift.

#### Scenario: A listed model's capability changes

- **WHEN** a model in the Desktop profile gains or loses `tool_search`
- **THEN** synchronization SHALL report a change to that client configuration
  file and rewrite the profile to match.

#### Scenario: A model declares tool search on another protocol

- **WHEN** a model declares `tool_search` on `responses` or `chat`
- **THEN** configuration validation SHALL reject that model.

### Requirement: Client verification runs outside a parent agent session

A client-depth check SHALL start each native client without the variables
through which a parent agent session configures its own client, and with the
user's own settings for that client, so that a check run from inside such a
session verifies the client's own files and credential command as the user's
own use of the client would.

#### Scenario: The check runs inside a Claude Code session

- **WHEN** `aigw check --depth client` runs inside a parent Claude Code
  session, which sets its marker, its host and session variables and its
  model mapping
- **THEN** the native client SHALL start without them
- **AND** it SHALL start with the user's own connection, runtime and
  provider-choice settings
- **AND** the result SHALL match the same check run outside that session.

### Requirement: Claude Code maps Anthropic model IDs to the provider's

AIGW SHALL write into Claude Code's settings one map from each Anthropic model ID that a listed model serves to that
model's ID. A model serves the Anthropic model ID that is its family, when that family begins with `claude-` and is
not the model's own ID. The listed models are the selected model and the first model of each other family of its
provider on the selected protocol, in configuration order. The map SHALL be a value that AIGW owns, records and
withdraws.

#### Scenario: Claude Code's picker rows reach the provider's channel

- **GIVEN** the selected provider serves Claude Fable 5.2 and Claude Opus 5.5 only as `claude-code-fable-5-2` and
  `claude-code-opus-5-5`, each in the family of the Anthropic model ID
- **WHEN** AIGW projects Claude Code's settings
- **THEN** the settings SHALL map `claude-fable-5-2` to `claude-code-fable-5-2` and `claude-opus-5-5` to
  `claude-code-opus-5-5`
- **AND** a model whose family is not an Anthropic model ID, or is its own ID, SHALL be absent from the map
- **AND** a map that would be empty SHALL not be written.

#### Scenario: The selected channel wins its family

- **WHEN** the selected model and another model of the selected provider share the family `claude-opus-5-5`
- **THEN** the map SHALL send `claude-opus-5-5` to the selected model's ID.

#### Scenario: A family's first model names its family

- **WHEN** the provider lists `claude-fable-5-2` before `claude-code-fable-5-2`, both in the family
  `claude-fable-5-2`, and neither is selected
- **THEN** the map SHALL hold no entry for `claude-fable-5-2`, since the listed model is the Anthropic ID itself.

#### Scenario: Only the map changes

- **WHEN** a command changes which models the provider lists, and nothing else of Claude Code's selection
- **THEN** that command SHALL rewrite Claude Code's settings with the new map.

#### Scenario: The map is withdrawn with Claude Code's projection

- **WHEN** AIGW withdraws Claude Code's projection
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, absent when they held none.

#### Scenario: The map changed outside AIGW

- **WHEN** the map in Claude Code's settings differs from what AIGW recorded and the next transition writes them
- **THEN** the transition SHALL refuse it as managed drift and write nothing.

#### Scenario: The settings hold the user's own map

- **GIVEN** Claude Code's settings hold a map that AIGW did not write
- **WHEN** AIGW projects a selection that needs a map
- **THEN** it SHALL write its map and keep the user's for the withdrawal
- **AND** while the selection needs no map, AIGW SHALL leave the user's map as it is.

#### Scenario: The selection no longer needs a map

- **WHEN** AIGW projects a selection that needs no map over settings that hold the map that AIGW wrote
- **THEN** the map SHALL return to what the settings held before AIGW wrote it.

### Requirement: Claude Desktop names the Anthropic model behind an entry

AIGW SHALL name, on each model entry of its Claude Desktop profile, the Anthropic model ID that the entry's model
serves, as the Claude Code map defines it, and no model on an entry that serves none. The name SHALL be part of the
profile that AIGW owns.

#### Scenario: Claude Desktop names the model behind each entry

- **WHEN** AIGW projects the Claude Desktop profile with `claude-code-fable-5-2` in the family `claude-fable-5-2`
- **THEN** that entry SHALL name `claude-fable-5-2` as the model behind it
- **AND** an entry whose ID is its family SHALL name none.

### Requirement: A record that the previous release reads keeps its shape

A record of a client's file that the previous published release reads strictly SHALL keep the fields and the digest
which that release reads. A setting that AIGW newly owns in that file outside what that digest covers SHALL have a
record of its own beside the first, whose reader keeps an entry that it does not know and leaves that entry's
setting as it is. Release acceptance SHALL roll the program back to the published predecessor with every newly
owned setting written.

#### Scenario: The previous release synchronizes after a rollback

- **GIVEN** the newer program wrote Claude Code's settings with a map
- **WHEN** the program is rolled back to the previous release and that release synchronizes
- **THEN** the synchronization SHALL succeed with Claude Code enabled
- **AND** the map SHALL stay in the settings as the newer program wrote it.

#### Scenario: A new setting lies inside what the record's digest covers

- **GIVEN** the newer program wrote each model's context window inside the provider entries of Hermes's file, which
  the digest of Hermes's record covers, and into Codex's model table, whose record holds the table's digest
- **WHEN** the program is rolled back to the previous release and that release synchronizes
- **THEN** the synchronization SHALL succeed with Hermes and Codex enabled, with no second record
- **AND** the previous release SHALL find both files its own and write its own form of them.

#### Scenario: The newer program returns after a rollback

- **WHEN** the newer program is activated again after the previous release synchronized, and synchronizes
- **THEN** it SHALL find the map its own, however the previous release re-indented the settings, and write the
  selection's map.

#### Scenario: The previous release withdrew the client

- **GIVEN** the previous release withdrew Claude Code after a rollback, which left the map and its record
- **WHEN** the newer program next projects or withdraws Claude Code, or is uninstalled
- **THEN** the map SHALL return to what the settings held before AIGW wrote it, and its record SHALL be removed.

#### Scenario: The record holds an entry of a later release

- **WHEN** the record of newly owned settings holds an entry that this program does not know
- **THEN** this program SHALL keep the entry when it writes the record and SHALL leave that entry's setting as it
  is.

### Requirement: Hermes runs each model at its declared context window

For every model of every provider entry that AIGW owns in Hermes's file, AIGW SHALL write the model's declared
context window as that model's `context_length`, under the entry's `models` as a mapping from the model's ID, in the
order in which AIGW lists the models. AIGW SHALL write no window on a provider entry or in Hermes's own `model`
block. Every reading of the models that an entry lists SHALL accept the mapping and the list of IDs that an earlier
release wrote.

#### Scenario: Every listed model carries its window

- **WHEN** AIGW projects Hermes's file
- **THEN** each provider entry that AIGW owns SHALL list each of its models as its ID with `context_length` set to
  the declared window, in AIGW's order
- **AND** no provider entry and no `model` block SHALL hold a `context_length` that AIGW wrote.

#### Scenario: A file that an earlier release wrote

- **GIVEN** Hermes's file lists an entry's models as a list of IDs, with AIGW's record of the file intact
- **WHEN** AIGW projects the file
- **THEN** it SHALL accept the file as its own and write the mapping.

#### Scenario: A model chosen in Hermes among mapped models

- **GIVEN** the file lists an entry's models as the mapping
- **WHEN** the operator selects another listed model in Hermes and `aigw sync` runs
- **THEN** Hermes's selection SHALL name that model, as it does for a list of IDs.

#### Scenario: The declared window changes

- **WHEN** the declared window of a listed model changes
- **THEN** `aigw sync --dry-run` SHALL plan a write of Hermes's file
- **AND** the command that changed the window SHALL write the new number for an enabled Hermes.

#### Scenario: An entry's models reordered outside AIGW

- **WHEN** the models of a provider entry that AIGW owns stand in Hermes's file in an order other than the one that
  AIGW wrote, the order of the configuration before the command, and nothing else of the entry changed
- **THEN** synchronization, a command that projects Hermes and its inspection SHALL refuse it as an outside edit
  without changing configuration or client files
- **AND** a command whose configuration changes the order of a provider's models SHALL write them in the new order.

### Requirement: A configured client program is never replaced

A client SHALL use its configured program whenever one is configured, and the program that discovery finds only when
none is configured, whose path AIGW then writes into the configuration. AIGW SHALL NOT replace a configured program
with another one in synchronization, the client list, a check or any other command, even when the configured program
is gone.

#### Scenario: The configured program is gone while discovery finds another

- **GIVEN** Codex is enabled with a configured program that is then removed, and discovery finds another `codex`
- **WHEN** `aigw sync` runs
- **THEN** the configuration SHALL keep the configured path
- **AND** AIGW SHALL NOT run the discovered program or write a file with it.

#### Scenario: No program is configured

- **WHEN** an enabled client has no configured program and discovery finds one
- **THEN** `aigw sync` SHALL write the discovered program's path into the configuration and synchronize the client with
  it.

#### Scenario: Setup takes the configured program first

- **WHEN** `aigw setup` decides which clients are installed while a client's configured program runs and discovery
  finds none
- **THEN** it SHALL count that client as installed.

#### Scenario: The client list shows the configured program

- **WHEN** `aigw client list` runs while a client's configured program is gone and discovery finds another
- **THEN** the client's row SHALL name the configured path and say that it is missing
- **AND** its JSON item SHALL carry that path as `executable` with `installed` false.

### Requirement: A missing program stops its client

When an enabled client's configured program is missing, or runs and cannot produce what AIGW writes for the client,
such as Codex's model table, every command that would write that client's files SHALL write none of them, its model
table or AIGW's record of them, apart from withdrawing a file that the change drops, SHALL write the rest of its
change, and SHALL exit non-zero with the cause and fix that the client's local check gives, never `aigw sync`.

#### Scenario: Codex's program is removed after a sync

- **GIVEN** `aigw sync` wrote Codex's configuration file, its model table and AIGW's record with a program that is
  then removed
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave the three files byte-identical and exit non-zero
- **AND** each SHALL name the missing program and the repair `aigw client edit codex --executable <path>`.

#### Scenario: The other clients continue

- **GIVEN** Claude Code's program is present and Codex's configured program is missing
- **WHEN** `aigw sync` runs
- **THEN** it SHALL synchronize Claude Code's settings
- **AND** it SHALL exit non-zero for Codex.

#### Scenario: A check finds a missing program

- **WHEN** `aigw check --depth local` finds that the configured program of Codex, Claude Code, Claude Desktop or Hermes
  is missing
- **THEN** that client's local depth SHALL fail with the missing program as its problem
- **AND** its repair SHALL be `aigw client edit <client> --executable <path>`.

#### Scenario: A runnable program ends the stop

- **WHEN** `aigw client edit codex --executable <path>` names a runnable program
- **THEN** AIGW SHALL synchronize Codex with that program.

#### Scenario: Codex cannot produce its model table

- **GIVEN** a sync wrote Codex's model table with a program whose file then changes into one that runs and prints no
  table
- **WHEN** `aigw sync` runs, and runs again
- **THEN** each run SHALL leave Codex's file, AIGW's record and the table byte-identical and exit non-zero
- **AND** each run and `aigw check --depth local` SHALL say that the configured executable cannot produce its model
  table, with the fix that names another Codex executable.

#### Scenario: A dropped file is withdrawn while the client is stopped

- **WHEN** `aigw client edit codex --target <file>` runs while Codex's configured program is missing
- **THEN** AIGW SHALL withdraw its settings, its record and its model table from the file that the edit drops
- **AND** it SHALL write nothing to the new file and exit non-zero with the stop.

#### Scenario: A selection meets a stopped client

- **WHEN** `aigw use codex <provider/model>` runs while Codex's configured program is missing
- **THEN** the selection SHALL be saved and Codex's files SHALL stay as they were
- **AND** the command SHALL keep its own report, its JSON fields among them, and then exit non-zero with the missing
  program, its fix and what became of Codex's files.

#### Scenario: A rollback meets a stopped client

- **WHEN** `aigw config rollback` runs while Codex's configured program is missing
- **THEN** `aigw.toml` SHALL be restored and Codex's files SHALL stay as they were
- **AND** the command SHALL exit non-zero, saying that `aigw.toml` was restored and that Codex's files were not
  written.

#### Scenario: A preview names the stopped clients

- **WHEN** `aigw sync --dry-run`, or the preview of another command that writes client files, runs while Codex's
  configured program is missing
- **THEN** it SHALL report its own plan of the other clients' files and then Codex's problem and fix, write nothing and
  exit non-zero.

#### Scenario: The repair refuses a program that cannot run

- **WHEN** `aigw client edit` or `aigw client enable` names an `--executable` that is not an existing regular file with
  an execute bit
- **THEN** it SHALL refuse before anything changes, without running that file.
