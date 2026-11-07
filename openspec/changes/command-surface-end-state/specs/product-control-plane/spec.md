## ADDED Requirements

### Requirement: Provider endpoints compose with independent products

AIGW SHALL treat every valid provider endpoint as an endpoint choice, whether
it is a direct provider HTTPS endpoint or an independently operated gateway.
AIGW MUST NOT identify, import, invoke, install, configure, diagnose, reload,
uninstall, or roll back the product behind that endpoint, and the external
gateway MUST NOT acquire AIGW state.

#### Scenario: A provider uses direct HTTPS

- **WHEN** an operator gives a provider a valid direct HTTPS endpoint
- **THEN** AIGW SHALL write that endpoint into client configuration files
  without requiring a local gateway
- **AND** `aigw check --depth client` SHALL exercise the selected client
  through that direct endpoint

#### Scenario: A provider uses loopback HTTP

- **WHEN** an operator gives a provider a valid loopback endpoint
- **THEN** AIGW SHALL treat it only as an external endpoint without product,
  fixed-port, path, or lifecycle assumptions

#### Scenario: Governed Codex deployment uses an external gateway

- **WHEN** a governed deployment gives providers gateway endpoints for one or
  more Codex models
- **THEN** AIGW SHALL write those endpoints by the same provider and model
  semantics used for direct HTTPS endpoints
- **AND** runtime evidence SHALL verify each selected client path without
  gateway lifecycle ownership or a fixed product, path, or port

#### Scenario: An external gateway is unavailable

- **WHEN** an independently operated gateway is absent or unhealthy
- **THEN** AIGW SHALL report only the selected endpoint's observed failure
- **AND** SHALL NOT install, start, stop, repair, or reconfigure the gateway

### Requirement: Providers, models, families and clients have distinct authority

A provider SHALL own an ID, a label, one endpoint per protocol and its Token.
A model SHALL belong to one provider and be named `provider/model`, where the
part after the first `/` is the provider's own model ID, byte for byte and
unique within that provider. A family SHALL group the models of one vendor
model. Only `[clients.<id>]` SHALL own a client's selected model, protocol,
authentication, `model_provider`, enabled intent and client-only options. Each
capability requires separate proof.

#### Scenario: A provider repeats a model ID

- **WHEN** a team file or `aigw.toml` gives one provider the same model ID twice
- **THEN** validation SHALL refuse it without changing any file.

#### Scenario: One provider serves different protocols per model

- **GIVEN** a provider has `responses` and `chat` endpoints
- **AND** two of its models declare different protocols
- **WHEN** a client resolves either model
- **THEN** only that model's declared protocols SHALL be eligible
- **AND** `anthropic`, `responses` and `chat` SHALL stay distinct protocols
- **AND** an endpoint of the provider SHALL NOT imply that a model works there.

#### Scenario: Responses text and Responses reasoning differ

- **GIVEN** a model can complete a basic `responses` text request
- **WHEN** its reasoning, tool, continuation, or compaction behavior has not
  been qualified for the selected client
- **THEN** AIGW SHALL report only the proven text capability
- **AND** SHALL NOT present the model as Codex-compatible or
  reasoning-qualified.

#### Scenario: The operator disables a client

- **WHEN** a client is disabled and AIGW's values are withdrawn from its
  configuration files
- **THEN** `aigw sync`, discovery and `aigw setup --from` SHALL keep it
  disabled
- **AND** its selected model, its provider's Token and other clients SHALL
  remain intact.

### Requirement: Every client selects a compatible model

Commands and a team file's new providers SHALL declare only model protocols
that the provider has an endpoint for; a model that a team file adds to a kept
provider without one SHALL stay unusable on that protocol until
`aigw provider edit --endpoint`. Every client's selection SHALL name one model
and one protocol that the model declares, the client speaks and the provider
serves. No global default or implicit inheritance SHALL take part in resolution,
readiness or client configuration files.

#### Scenario: A command declares a protocol that the provider lacks

- **WHEN** a command, or a team file for a provider that it adds, declares a
  model protocol for which the provider has no endpoint
- **THEN** validation SHALL reject it before any write
- **AND** the error SHALL name the model, the provider and the missing
  protocol.

#### Scenario: A selection needs an endpoint

- **WHEN** configuration persistence receives a client's selection whose
  provider has no endpoint for the selected protocol
- **THEN** it rejects the change and preserves the existing configuration file.

#### Scenario: Check every enabled client's model

- **WHEN** Claude Code and Codex are enabled with different selected models
- **THEN** `aigw check` SHALL derive each AIGW-owned authenticated request from
  that client's selected protocol and authentication
- **AND** SHALL NOT inspect an unselected model as a fallback
- **AND** MAY share one authenticated request between clients only when
  provider, endpoint, protocol, depth and any carried model ID are identical.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `aigw status --json` reports a client whose provider has an endpoint
  for the selected protocol
- **THEN** it SHALL report that the endpoint is configured
- **AND** SHALL NOT represent that configuration fact as endpoint or transport
  health.

### Requirement: One configuration file holds providers, models and families

AIGW SHALL keep its configuration in one credential-free `aigw.toml` with
`version = 8`, holding providers with their models, families, recommendations
and clients. Commands SHALL take their configuration only from that file; only
the one-time import of `aigw sync` and `aigw self rollback` SHALL read
`config.toml`. Array order SHALL be display order, and decoding SHALL refuse a
field that schema 8 does not define. A team file SHALL use the same format
without `[clients]`.

#### Scenario: Lists follow the order of the file

- **WHEN** `aigw.toml` lists providers, and the models of one provider, in an
  order that is not alphabetical
- **THEN** `aigw provider list`, `aigw model list`, their JSON output and
  `aigw config export` SHALL keep that order.

#### Scenario: A retired field is present

- **WHEN** `aigw.toml` contains an `order`, `purpose` or `lifecycle` field
- **THEN** loading it SHALL fail without changing any file.

#### Scenario: A model is deprecated

- **WHEN** a team file declares `deprecated = true` for a model
- **THEN** a client that selects it SHALL keep its selection
- **AND** `aigw model list` SHALL mark it, and `aigw model search` SHALL
  report it as `deprecated` while its provider's catalog lists it
- **AND** a recommendation that names it SHALL fail validation
- **AND** only a team file, or the one-time import, SHALL set `deprecated`.

#### Scenario: A model is its own family

- **WHEN** a model omits `family`
- **THEN** it SHALL be its own family and need no `[[families]]` entry.

#### Scenario: A file carries a credential

- **WHEN** `aigw.toml` or a team file contains a Token, password,
  authorization header, API key or equivalent credential field
- **THEN** AIGW SHALL reject it without changing local configuration.

### Requirement: Models declare their protocols and capabilities

A model's `protocols` SHALL name the endpoints of its provider on which AIGW
may use it, and map each protocol to the capabilities qualified there. A
protocol SHALL come from the client that uses the model, from `--protocol`,
from a team file or from the provider's only endpoint, never from a catalog or
a model ID. Only `aigw model add`, `aigw model edit`, a team file and the
one-time import SHALL set a capability, and changing a protocol's endpoint
SHALL clear those on that protocol.

#### Scenario: A capability goes to each protocol that admits it

- **WHEN** `aigw model edit <provider/model> --capability tool_search` runs for
  a model that declares `anthropic` and `responses`
- **THEN** the given set SHALL replace the model's capabilities, so that it
  declares `tool_search` on `anthropic` and nothing on `responses`
- **AND** `--capability ""` SHALL clear every capability of the model
- **AND** the client configuration files that list the model SHALL change in
  the same transaction.

#### Scenario: No protocol admits a capability

- **WHEN** `aigw model add` or `aigw model edit` gives
  `--capability tool_search` for a model that does not declare `anthropic`
- **THEN** it SHALL refuse before any write.

#### Scenario: A protocol's endpoint changes

- **WHEN** `aigw provider edit <provider> --endpoint anthropic=<url>` changes
  that protocol's endpoint
- **THEN** the capabilities of its models on `anthropic`, and on no other
  protocol, SHALL be cleared in the same transaction
- **AND** the result and its `--dry-run` preview SHALL report each one.

### Requirement: Adding a model applies the catalog check and selects nothing

`aigw model add <provider/model>` SHALL apply the catalog check that
`model-selection` defines and SHALL select the model for no client. Its
`--protocol` SHALL be optional only for a provider with one endpoint. Without
`--family` it SHALL follow the family rule of `model-selection`, and an unknown
`--family` SHALL create that family, labelled from the model.

#### Scenario: Adding a model selects nothing

- **WHEN** `aigw model add <provider/model>` adds a model
- **THEN** no client's selection SHALL change
- **AND** client configuration files that list that provider's models SHALL
  change in the same transaction.

#### Scenario: The endpoints decide whether a protocol is needed

- **WHEN** `aigw model add <provider/model>` runs without `--protocol`
- **THEN** a provider whose only endpoint is `responses` SHALL give the model
  `responses`
- **AND** a provider with several endpoints SHALL refuse before any write,
  naming `--protocol` and its protocols.

#### Scenario: An unknown family is created

- **WHEN** `aigw model add` or `aigw model edit` names a `--family` that does
  not exist
- **THEN** AIGW SHALL create that family in the same transaction, labelled from
  the model.

### Requirement: Models are edited and removed explicitly

`aigw model edit` SHALL change only what it names and replace each repeatable
set that it is given. `aigw model remove` SHALL refuse a model that a client
selects, enabled or disabled, and SHALL drop and report the recommendations that
name it. Each SHALL write in one transaction.

#### Scenario: Edit replaces a model's protocols

- **WHEN** `aigw model edit <provider/model>` with `--protocol responses` and
  `--protocol chat` runs for a model that declares `anthropic`
- **THEN** the model SHALL declare exactly `responses` and `chat`, and keep its
  label and family.

#### Scenario: Edit would take a protocol from a client

- **WHEN** the protocols that `aigw model edit` gives omit the protocol of a
  client's selection
- **THEN** it SHALL refuse before any write
- **AND** its Fix SHALL be `aigw use <client> <provider/model> --protocol <p>`,
  which moves the client, after which the edit can run.

#### Scenario: Remove a model that a client selects

- **WHEN** `aigw model remove <provider/model>` names a model that a client
  selects, enabled or disabled
- **THEN** it SHALL refuse before any write
- **AND** its Fix SHALL be `aigw use <client> <provider/model>` with that client
  filled in.

#### Scenario: Remove a recommended model

- **WHEN** `aigw model remove` removes a model that a recommendation names
- **THEN** the recommendation SHALL drop it in the same transaction, and its
  first remaining alternative SHALL become primary when the primary is removed
- **AND** the result and its `--dry-run` preview SHALL name each client whose
  recommendation changes.

### Requirement: Model list reads only the configuration

`aigw model list [query]` SHALL show each added model as `provider/model` with
its label, its protocols and the clients that select it, marking deprecated
models, without network access or a Token read. Repeatable `--client` SHALL
narrow the list to the models that a client can use, and `--protocol` to the
models that declare one protocol.

#### Scenario: List models without network access

- **WHEN** `aigw model list` runs while every provider is unreachable and the
  credential store is locked
- **THEN** it SHALL list every added model in configuration order
- **AND** SHALL read no Token and send no request.

#### Scenario: Narrow the list to one client

- **WHEN** `aigw model list --client claude` runs
- **THEN** it SHALL list only the models that declare a protocol Claude Code
  speaks.

### Requirement: Providers are removed with their models and Tokens

`aigw provider remove <provider>` SHALL delete the provider, its models, its
Token and its balance credentials in one transaction, and SHALL refuse while a
client selects one of its models, enabled or disabled. Recommendations that name
its models SHALL be dropped and reported.

#### Scenario: Remove an unused provider

- **WHEN** `aigw provider remove ucloud` runs and no client selects a `ucloud`
  model
- **THEN** the provider, its models, its Token and its balance credentials SHALL
  be deleted
- **AND** each recommendation that named a `ucloud` model SHALL be dropped and
  reported
- **AND** client configuration files that list `ucloud` models SHALL change in
  the same transaction
- **AND** a configuration left with no model SHALL stay valid.

#### Scenario: A client selects one of its models

- **WHEN** a client, enabled or disabled, selects a model of the provider being
  removed
- **THEN** the command SHALL refuse before any change
- **AND** its Fix SHALL be `aigw use <client> <provider/model>` with that client
  filled in.

#### Scenario: A read-only backend supplies the credentials

- **WHEN** `aigw provider remove <provider>` runs with the read-only environment
  backend
- **THEN** it SHALL remove the configuration and name the variable to unset.

## MODIFIED Requirements

### Requirement: Provider-neutral configuration

AIGW SHALL model providers, models, families, clients, endpoints and
authentication without provider-specific hacks, named gateways, topology or
global fallback. Only the current credential-free schema SHALL execute.
Offline validation SHALL serve persistence and `aigw setup --from`, report the
first failure deterministically, and derive readiness from authentication
ownership.

#### Scenario: Diagnose several configuration problems

- **WHEN** the same configuration contains several invalid entries
- **THEN** repeated validation SHALL identify the same first problem
- **AND** the supplied configuration, including unset collections, SHALL remain
  unchanged.

### Requirement: Independent product authority

AIGW SHALL own only its configuration, provider Tokens, each client's selected
model and the values it writes into client configuration files. It MUST NOT
carry traffic, infer or manage an endpoint implementation, control unrelated
applications, or rewrite client-private state. Any conforming `responses` URL
MAY be a provider endpoint and MUST be diagnosed only through its declared
protocol when a client uses a model on it.

#### Scenario: Team configuration selects a Responses endpoint

- **WHEN** a team file contains a `responses` endpoint
- **THEN** `aigw setup --from` SHALL remain independent of its implementation
  and lifecycle
- **AND** readiness SHALL probe it only for an installed client whose selected
  model uses it.

#### Scenario: Native Codex CLI and Desktop share a home

- **WHEN** AIGW discovers the native Codex Home
- **THEN** it SHALL write its marked selection into Codex's shared
  `config.toml` without editing application-managed history or GUI state.

#### Scenario: Codex uses the selected endpoint

- **WHEN** a provider has an implementation-neutral `responses` endpoint
- **THEN** AIGW SHALL treat it as an ordinary endpoint
- **AND** Claude Code SHALL continue using its independently selected
  `anthropic` endpoint.

#### Scenario: Compose with an external Responses service

- **WHEN** an operator configures the HTTP endpoint of an external service as
  a provider endpoint
- **THEN** AIGW SHALL treat it exactly as an external endpoint
- **AND** SHALL NOT acquire lifecycle or state ownership over that service.

### Requirement: Declarative ordinary provider extension

An ordinary provider SHALL be added through token-free provider, endpoint and
model data, from a team file or `aigw provider add` and `aigw model add`, and an
optional balance probe. Adding it MUST NOT require a provider-specific command,
client configuration branch, installer case, service manager, core dependency,
or edits to an existing client adapter, release path, or repository policy.

#### Scenario: A synthetic provider is imported

- **WHEN** a valid team file adds one provider with supported protocol
  endpoints and models
- **THEN** every applicable supported client can select its models through the
  ordinary configuration and client configuration path
- **AND** architecture verification proves no provider-named core branch or
  additional product owner was introduced

#### Scenario: An endpoint needs Responses compatibility

- **WHEN** a provider endpoint is an external compatibility service
- **THEN** AIGW treats it as an ordinary endpoint
- **AND** does not install, configure, start, stop, or verify that service.

### Requirement: Provider identity is not client behavior

AIGW MUST NOT encode a provider name, alias, or product identity to enable an
unrelated Codex storage, replay, authentication, or compatibility behavior.
What AIGW writes into client configuration files SHALL use only explicit
capabilities that the client's own contract supports.

#### Scenario: An endpoint needs non-default Responses behavior

- **WHEN** a provider endpoint has a distinct storage or replay expectation
- **THEN** AIGW SHALL record only an explicit supported client capability or
  endpoint choice
- **AND** it SHALL NOT rename the provider to Azure or another identity

### Requirement: Optional provider-native diagnostics

Provider balance probes SHALL be optional leaf capabilities behind one
provider-neutral contract. Setup, `use`, `sync` and every `check` depth MUST
remain functional without them. `aigw provider balance` SHALL report every
provider unless providers are named, and a provider without a probe or with a
failed probe SHALL NOT hide the others.

#### Scenario: No diagnostic is present

- **WHEN** a build contains no balance probe for a provider
- **THEN** `use`, `sync` and `check` SHALL continue normally for that provider
- **AND** only `aigw provider balance` SHALL report that the provider has no
  balance probe, and that report SHALL NOT be a failure

#### Scenario: One of three balance probes fails

- **WHEN** `aigw provider balance` runs without arguments for three providers
  and one probe fails
- **THEN** it SHALL report the other two balances and name the failed
  provider's fix
- **AND** the command SHALL exit 1.

### Requirement: Token rotation is credential-scoped

AIGW SHALL replace only the selected provider's Token, after `secret-storage`'s
validation. Rotation MUST leave providers, models, families, client selections,
client configuration files, ownership sidecars and client-owned credentials
unchanged. `aigw provider add`, `aigw provider rotate`, `aigw use` and setup
SHALL use the same Token replacement and compensation owner. Compensation SHALL
inspect the written Token, preserve a different observed value, and report
incomplete storage recovery.

#### Scenario: The next helper invocation observes rotation

- **WHEN** `aigw provider rotate` successfully replaces the selected provider's
  Token
- **THEN** a matching credential command in a client configuration file reads
  the replacement on its next call
- **AND** rotation invokes no client and changes no client-owned credentials
- **AND** its result does not promise immediate refresh by a running client.

#### Scenario: A newer Token appears before compensation

- **WHEN** compensation observes a Token different from the one it wrote
- **THEN** it preserves the observed Token and reports the ownership conflict
- **AND** client-owned credentials remain unchanged.

#### Scenario: The operator cancels validation

- **WHEN** `aigw provider rotate` is cancelled during Token validation
- **THEN** validation observes the command cancellation
- **AND** no replacement Token is persisted.

### Requirement: Model use follows protocol and capability

AIGW SHALL accept a model independently of client branding, using the protocols
it declares and its verified capabilities. The client, `--protocol` or a team
file SHALL choose among protocols; AIGW SHALL NOT prefer one. An independently
selected compatibility service MAY supply a missing protocol without becoming
part of AIGW's runtime. Displayed mappings SHALL preserve the identity of the
requested and serving models. A successful text request SHALL establish only
text connectivity.

#### Scenario: A non-Anthropic model serves Claude

- **WHEN** a client explicitly selects a non-Anthropic model through a
  compatible `anthropic` endpoint
- **THEN** AIGW SHALL preserve that provider and model identity in its output
- **AND** support claims SHALL reflect tested streaming, tool results,
  continuation, and model-specific limits.

#### Scenario: Codex uses an independent Responses endpoint

- **WHEN** a client explicitly selects a model through an independent
  `responses` endpoint
- **THEN** validation SHALL verify its actual input, event, and tool contracts
- **AND** missing compaction or hosted tools SHALL have explicit outcomes
- **AND** existing conversation metadata and native model selections remain
  unchanged.

#### Scenario: A provider exposes a smaller capability set

- **WHEN** an endpoint does not support a capability required by a client
  journey
- **THEN** the journey SHALL report the specific unsupported capability
- **AND** configuration or a renamed model SHALL NOT be presented as full
  acceptance.

### Requirement: Diagnostic quota and throttling remain distinct

An authenticated diagnostic SHALL use explicit provider error evidence, not
HTTP status alone, to distinguish exhausted provider credit or quota from
transient rate or concurrency limits. Classification SHALL NOT switch a
client's model or retry a request automatically.

#### Scenario: A provider reports hard quota exhaustion with HTTP 429

- **WHEN** a provider diagnostic receives HTTP 429 with an unambiguous
  exhausted quota code or insufficient balance evidence
- **THEN** AIGW SHALL report non-retryable quota exhaustion for that provider
  and give provider-scoped recovery guidance

#### Scenario: A provider reports transient throttling with HTTP 429

- **WHEN** a provider diagnostic receives HTTP 429 with rate-limit or ambiguous
  concurrency-quota evidence
- **THEN** AIGW SHALL report retryable throttling rather than exhausted credit
- **AND** the diagnostic SHALL have made only its single permitted request.

### Requirement: Model discovery reports catalog evidence only

`aigw model search [query]` SHALL read the authenticated catalog of every
provider, or of each that repeatable `--provider` names, and report each
matching model once under its provider as `added`, `available`, `missing` or
`deprecated`. It SHALL share one observation path with the catalog checks of
`aigw use` and `aigw model add`, and a catalog SHALL count only as evidence of
what its provider lists when read, never as inference reachability or client
readiness.

#### Scenario: Search two providers

- **WHEN** `aigw model search sonnet --provider dmxapi --provider ucloud` runs
- **THEN** it SHALL read each distinct protocol source once, concurrently,
  within the existing 15-second bound
- **AND** each provider SHALL appear once, with each matching model on one line
  as `provider/model` with its state, even when several sources list it
- **AND** JSON output SHALL keep which protocol source listed each model.

#### Scenario: A configured model appears in the catalog

- **WHEN** an authenticated catalog lists the ID of an added model
- **THEN** `aigw model search` reports that model as `added`
- **AND** performs no inference or client invocation.

#### Scenario: A provider lists a new model

- **WHEN** a catalog read observes a model ID that the configuration does not
  name
- **THEN** `aigw model search` SHALL report it as `available`
- **AND** the configuration, recommendations, client selections and client
  configuration files SHALL stay unchanged until an operator adds it, and only
  a team file or the removal of a model SHALL change a recommendation.

#### Scenario: A model is absent from a successfully observed catalog

- **WHEN** every protocol source of a provider is read completely, and none
  lists the ID of an added model
- **THEN** `aigw model search` reports that model as `missing` rather than
  unavailable, even when it is deprecated
- **AND** AIGW SHALL NOT treat it as withdrawn, renamed or incompatible: it
  SHALL keep the model and every client's selection of it and infer no rename
  from a newly listed ID.

#### Scenario: A provider is read on one protocol but not another

- **WHEN** one protocol source of a provider can be read and another cannot
- **THEN** search SHALL show the models that it got and name the failed source
- **AND** it SHALL report no model of that provider as `missing`
- **AND** the command SHALL exit 1.

#### Scenario: Catalog observation fails

- **WHEN** the Token cannot be observed, the endpoint is absent, the request
  fails, or the response is incomplete or oversized
- **THEN** that provider has no established model membership, and SHALL get
  one line with its reason and fix while the other providers still print
- **AND** the command SHALL exit 1.

#### Scenario: A catalog response is ambiguous

- **WHEN** a successful HTTP response has a null model list, repeats that list,
  or repeats an identifier member within one item
- **THEN** that provider has no established model membership, and
  `aigw model search` SHALL NOT report its added models as `missing`
- **AND** an empty list, the supported identifier shapes and other provider
  fields SHALL still produce a successful observation.

## REMOVED Requirements

### Requirement: Independent product composition

**Reason**: Its scenarios name Accounts, which providers replace.

**Migration**: Read "Provider endpoints compose with independent products".

### Requirement: Models, Routes, and Client Bindings have distinct authority

**Reason**: Providers, models, families and clients replace Accounts, Routes,
canonical Models and Client Bindings.

**Migration**: Read "Providers, models, families and clients have distinct
authority", with selection rules in `model-selection`, team-file
recommendations in `progressive-team-onboarding`, an unselected client in
`client-adapters`, and the import of models sharing an ID in
`credential-continuity`.

### Requirement: Schema replacement is explicit and reversible

**Reason**: `aigw sync` imports a schema 6 configuration once, replacing
`aigw config migrate`.

**Migration**: `credential-continuity` owns the import, its refusals and its
rollback, and "One configuration file holds providers, models and families"
the schema 8 file.

### Requirement: Route and Client Binding compatibility

**Reason**: It mixed configuration compatibility with check, status, doctor
and setup output that now have their own owners.

**Migration**: Read "Every client selects a compatible model", with
`aigw check` in `cli-readiness`, `aigw use` in `model-selection`, team files in
`progressive-team-onboarding` and earlier schemas in `credential-continuity`.

### Requirement: Provider catalog evolution is observed before admission

**Reason**: The search states `added`, `available`, `missing` and `deprecated`
replace the admission states.

**Migration**: Read "Model discovery reports catalog evidence only", and
"Recommendations are advisory" in `model-selection` for a recommended model
that stops working.

### Requirement: Stored capabilities follow their recorded Route

**Reason**: Each model declares its capabilities per protocol in `aigw.toml`,
replacing the separate capability component.

**Migration**: Read "Models declare their protocols and capabilities", and
`credential-continuity` for the one-time import of schema 6 capabilities.

## RENAMED Requirements

- FROM: `### Requirement: Model admission follows protocol and capability`
- TO: `### Requirement: Model use follows protocol and capability`
