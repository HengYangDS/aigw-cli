# client-adapters Specification

## Purpose

Define how an admitted client's adapter joins AIGW: its admission, the extension
boundary that every adapter shares, the client surfaces it serves, its native
provider declarations and how the control plane converges each client.

## Requirements

### Requirement: Independently admitted native clients

Codex and Claude Code SHALL be independent Adapters owning discovery, projection, authentication, rollback,
verification, status, and withdrawal of AIGW state. Provider-Token credential commands MUST use an absolute qualified
reader, never store Tokens in client files, and never rely on a package-manager CLI link. A client's own sign-in and
preferences SHALL remain client-owned. A new client MUST add an Adapter without changing provider policy or other
Adapters.

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

- **WHEN** a version 9 configuration or team file names, in `[clients.codex]` or a Codex recommendation, an explicit
  native provider through `model_provider` or the client's own authentication through `authentication`
- **THEN** loading the file SHALL refuse that field, which schema 9 does not define
- **AND** Codex SHALL authenticate only through AIGW's own provider, whose credential command invokes the selected
  qualified reader with the fingerprint of Codex's selected provider and endpoint
- **AND** no plaintext Token SHALL be written to public configuration

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

### Requirement: Composable extension boundary

AIGW SHALL keep configuration and client projection separate from API traffic. Endpoints and models SHALL enter as
provider data. New clients SHALL require complete Adapters, unsupported credential contracts SHALL require admission,
and incompatible wire behavior SHALL remain in a separate data plane. Gateways SHALL remain optional endpoints.
Dependencies MAY be admitted only when total owned complexity falls.

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

- **WHEN** an AWS model service exposes an admitted client protocol but needs the client's own credential chain and
  request signing
- **THEN** AIGW SHALL offer no configuration that hands the client's authentication to that chain, since schema 9
  defines no `authentication` key in a client or a recommendation
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

### Requirement: Codex lists the models that its table describes

In the model table that AIGW projects for Codex, AIGW SHALL list the models that a client lists for the selected
provider and protocol, the selected model and the first model of each other family in configuration order, wherever
the client's bundled table holds the model's own entry or its family's. Every other entry that the client lists SHALL
stay in the table, hidden from Codex's picker. An entry that the client hides SHALL stay hidden, and so SHALL an alias
of it.

#### Scenario: The picker lists the configured families

- **GIVEN** the selected provider serves `gpt-6.2-sol-cdx` in the family `gpt-6.2-sol`, and `gpt-6.1-terra` and
  `deepseek-v4.1` each as its own family, on the Responses protocol
- **AND** the client's table lists `gpt-6.2-sol`, `gpt-6.1-terra` and `gpt-5.5` and hides `codex-auto-review`
- **WHEN** AIGW projects Codex's table with `gpt-6.2-sol-cdx` selected
- **THEN** the table SHALL list the alias `gpt-6.2-sol-cdx` and the entry `gpt-6.1-terra`
- **AND** SHALL hide `gpt-6.2-sol` and `gpt-5.5` and keep `codex-auto-review` as the client has it
- **AND** SHALL hold no entry for `deepseek-v4.1`.

#### Scenario: A listed model's entry is one that the client hides

- **WHEN** the entry that describes a listed model, its own or its family's, is hidden in the client's table
- **THEN** the model's entry SHALL stay hidden.

#### Scenario: The table describes no listed model

- **WHEN** the client's table describes none of the listed models
- **THEN** the projected table SHALL list no entry and SHALL keep every entry of the client's table, hidden.
