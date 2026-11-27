## MODIFIED Requirements

### Requirement: The client list shows each client's state

`aigw client list` SHALL list every client that AIGW supports, each with the program that it uses, as declared,
resolved from its declared extensions directory or found by discovery, or that it is not installed; the extensions
directory that it declares; whether it is enabled; and its selected `provider/model` or that it has none. Its JSON item
SHALL carry the program as `program` and a declared directory as `extensions`.

#### Scenario: List the clients

- **GIVEN** Codex is installed, enabled and selects `dmxapi/gpt-6.1-sol`, and
  Hermes is not installed
- **WHEN** `aigw client list` runs
- **THEN** Codex's row SHALL show its program path, that it is enabled and
  `dmxapi/gpt-6.1-sol`
- **AND** Hermes's row SHALL show that it is not installed, and Claude Code and
  Claude Desktop SHALL have rows too.

#### Scenario: A client declares an extensions directory

- **GIVEN** Claude Code declares `extensions = "<dir>"`, whose highest version of Claude Code's extension holds its
  program
- **WHEN** `aigw client list --json` runs
- **THEN** Claude Code's item SHALL carry `<dir>` as `extensions` and the resolved program's path as `program`, and no
  `executable`
- **AND** its human row SHALL name the declared directory beside that program
- **AND** when no installed version holds its program, the item SHALL carry `extensions` with `installed` false and no
  `program`.

### Requirement: Forwarding belongs to one client

`aigw client edit <client> --via <root>` MAY send one client's Responses traffic through a forwarding service at that
root. AIGW SHALL compose the destination from the selected provider's Responses endpoint as
`<root>/<scheme>/<authority>/<base path>`; selection, the credential reader, client files, previews and rollback SHALL
use it and keep the provider's Token, the model, other clients and unmanaged content. AIGW SHALL NOT manage the service
or carry requests.

#### Scenario: One client forwards to an installed endpoint

- **WHEN** an enabled Codex whose model belongs to a provider with the Responses endpoint
  `https://www.dmxapi.cn/v1` gets `aigw client edit codex --via http://127.0.0.1:8792`
- **THEN** a credential-free preview SHALL show only Codex's change, its base URL becoming
  `http://127.0.0.1:8792/https/www.dmxapi.cn/v1`
- **AND** the change SHALL keep the provider and the other clients
- **AND** the next sync SHALL keep the forwarding.

#### Scenario: One upstream has one spelling

- **WHEN** the provider's Responses endpoint is `https://API.Example.com:443/v1/`
- **THEN** the destination SHALL be `<root>/https/api.example.com/v1`
- **AND** an endpoint with another port SHALL keep it, as in `<root>/https/api.example.com:8443/v1`.

#### Scenario: An upstream outside the grammar

- **WHEN** a forwarded client would reach a Responses endpoint with a query, a fragment, user information,
  percent-encoding, an empty or dot path segment, or `http` to a host that is not a loopback host
- **THEN** the command SHALL refuse before any change, naming `aigw client edit <client> --direct`.

#### Scenario: The root is checked

- **WHEN** `--via` names a URL that is neither `https` nor `http` of a loopback host, or that carries user
  information, a query or a fragment
- **THEN** the command SHALL refuse before any change.

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

A client's forwarding SHALL stay through a change of model, provider or provider endpoint, its destination following
the new upstream. A command's change to an upstream that the grammar cannot name SHALL be refused, naming
`aigw client edit <client> --direct`; a model chosen in the client SHALL be followed without the forwarding. Retry
budgets SHALL stay through a change within the provider and SHALL be cleared, with the result saying so, on a change
of provider.

#### Scenario: A client changes model within its provider

- **WHEN** a client with forwarding and retry budgets selects another model of
  the same provider on the same protocol
- **THEN** its forwarding and retry budgets SHALL stay.

#### Scenario: A client changes protocol within its provider

- **WHEN** a forwarded client with retry budgets selects a model of the same provider on a protocol other than
  Responses
- **THEN** the selection SHALL be refused before any change, naming `aigw client edit <client> --direct`
- **AND** a selection without forwarding SHALL keep its retry budgets through the same change.

#### Scenario: A client moves to another provider

- **WHEN** `aigw use` moves a client with forwarding and retry budgets to a Responses model of another provider whose
  endpoint the grammar names
- **THEN** the forwarding SHALL stay, its destination naming the new provider's upstream
- **AND** the retry budgets SHALL be cleared, and the result SHALL say so.

#### Scenario: The provider's endpoint changes

- **WHEN** `aigw provider edit <provider> --endpoint responses=<url>` changes the endpoint that a forwarded client
  reaches
- **THEN** that client's destination SHALL follow the new endpoint at the next synchronization, or the edit SHALL be
  refused before any change when the grammar cannot name it
- **AND** its retry budgets SHALL stay.

#### Scenario: The client chose a model whose upstream the grammar cannot name

- **WHEN** a synchronization follows a model chosen in a forwarded client whose upstream is another protocol than
  Responses or an endpoint outside the grammar
- **THEN** it SHALL follow the model and drop the client's forwarding
- **AND** its result SHALL say so with `forwarding_cleared`.

#### Scenario: The operator connects a client directly

- **WHEN** `aigw client edit <client> --direct` runs for a forwarded client
- **THEN** the client SHALL drop its forwarding, and its files SHALL again name its provider's endpoint.

#### Scenario: A check of another provider's model inherits no budget

- **WHEN** `aigw check <client> --model <provider/model>` names a model of
  another provider
- **THEN** the check SHALL NOT use the client's retry budgets.

## ADDED Requirements

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
- **THEN** `aigw check` SHALL derive each AIGW-owned authenticated request from that client's selected protocol and its
  provider's Token
- **AND** SHALL NOT inspect an unselected model as a fallback
- **AND** MAY share one authenticated request between clients only when
  provider, endpoint, protocol, depth and any carried model ID are identical.

#### Scenario: Inspect an endpoint address without probing it

- **WHEN** `aigw status --json` reports a client whose provider has an endpoint
  for the selected protocol
- **THEN** it SHALL report that the endpoint is configured
- **AND** SHALL NOT represent that configuration fact as endpoint or transport
  health.
