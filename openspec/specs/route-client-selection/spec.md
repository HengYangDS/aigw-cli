# route-client-selection Specification

## Purpose

Define deterministic client selection from explicit operator intent, reusable
Routes, and independent Client Bindings without inference from names or provider
details.

## Requirements

### Requirement: Route selection is explicitly client-scoped

A Route SHALL identify one Account, one canonical Model, its exact upstream
identifier, and its admitted protocol interfaces independently of client state.
A Client Binding SHALL select one compatible Route for one client. Non-interactive
selection SHALL require both identities; interactive selection MAY prompt for a
missing client or Route when exactly the admitted choices are shown.

#### Scenario: Select a Route for one client

- **WHEN** the operator runs `aigw use --for <client> <route>`
- **THEN** AIGW SHALL update only that client's binding and owned projection
- **AND** SHALL preserve every other client's selection, files, and credentials.

#### Scenario: One Route serves several clients

- **GIVEN** one Route exposes interfaces admitted by several clients
- **WHEN** the operator selects it independently for those clients
- **THEN** AIGW SHALL retain one Route and one Client Binding per client
- **AND** SHALL NOT duplicate the Route or introduce a global selection.

### Requirement: Connectivity and verification use explicit Route semantics

`aigw test` SHALL test selected client endpoints without changing bindings.
`aigw verify` SHALL perform a real bounded client request. An explicit `--route`
override SHALL require exactly one explicit client and SHALL NOT mutate its
stored binding. A retry that either command names SHALL keep the same client
and Route. A private verification projection SHALL reach the operator's
user-owned files without copying or owning them.

#### Scenario: Test an explicit Route

- **WHEN** the operator runs `aigw test --for <client> --route <route>`
- **THEN** only that Route's resolved endpoint and credential boundary are tested
- **AND** no client configuration or selection changes.

#### Scenario: Verify an explicit Route

- **WHEN** the operator runs `aigw verify --for <client> --route <route>`
- **THEN** AIGW SHALL invoke that client through an isolated projection
- **AND** SHALL require successful completion from the real client
- **AND** an HTTP probe alone SHALL NOT establish the client result.

#### Scenario: Verify all enabled clients

- **WHEN** the operator runs `aigw verify --for all`
- **THEN** every enabled Client Binding SHALL be verified independently
- **AND** `--route` SHALL be rejected because no single Route can override several clients.

#### Scenario: Verification leaves operator extensions out

- **WHEN** `aigw verify` runs a real client for one Route
- **THEN** the client SHALL run in a private home or its native isolation mode
  without starting the operator's MCP servers, plugins, hooks, notifications or
  memory providers
- **AND** an operator extension that cannot start or authorize there SHALL NOT
  decide the verification result.

#### Scenario: A failed explicit-Route command names its retry

- **WHEN** `aigw verify --for <client> --route <route>` fails, or
  `aigw test --for <client> --route <route>` defers that Route to `aigw verify`
- **THEN** the command it names SHALL be
  `aigw verify --for <client> --route <route>`
- **AND** the same failure without `--route` SHALL name
  `aigw verify --for <client>`.

#### Scenario: The operator's Codex catalog is relative to its configuration

- **WHEN** the operator's own `model_catalog_json` is a path relative to the
  Codex configuration that names it
- **THEN** the verification projection SHALL name the same catalog file by its
  absolute path
- **AND** the catalog and the operator's configuration SHALL stay unchanged, and
  AIGW SHALL NOT take ownership of the catalog.

#### Scenario: A rooted Windows catalog reference keeps its drive

- **WHEN** on Windows the operator's own `model_catalog_json` is rooted without
  a drive, such as `\catalogs\models.json`
- **THEN** the verification projection SHALL name that path on the drive of the
  configuration that names it
- **AND** a reference that names a drive without a root, such as
  `D:models.json`, SHALL stay unchanged.

### Requirement: Ambiguous or incompatible selection fails closed

AIGW SHALL NOT infer a client, protocol, capability, canonical Model, or Route
from identifiers, model families, endpoint presence, another client's binding,
or declaration order.

#### Scenario: A Route is incompatible with the requested client

- **WHEN** the selected Route has no protocol admitted by the requested client
- **THEN** the command SHALL fail before credential access, network access, or projection
- **AND** SHALL identify the client, Route, and missing compatible protocol.

#### Scenario: A client has no selected Route

- **WHEN** an operation requires a client whose binding has no Route
- **THEN** AIGW SHALL report that exact missing selection
- **AND** SHALL recommend `aigw use --for <client> <route>`
- **AND** SHALL NOT substitute another client's Route.

### Requirement: Selection owns one guarded transaction

Selection SHALL validate any required Account Token and reuse the shared
credential replacement and synchronization owners. Failure or cancellation
before commit SHALL compensate only writes owned by that attempt. Rendering
failure after commit SHALL not undo committed configuration or credentials.

#### Scenario: Selection fails after Token storage

- **WHEN** client convergence or configuration persistence fails
- **THEN** unchanged credential postimages and a newly persisted automatic
  backend choice SHALL be compensated
- **AND** any compensation error SHALL remain visible with the original failure.

#### Scenario: Re-select the active Route

- **WHEN** the selected Route, projection, and required credential already match
- **THEN** selection SHALL succeed as an observable no-op
- **AND** SHALL not rewrite files, credentials, or verification checkpoints.

### Requirement: Provider preference is advisory control-plane input

A recommendation MAY order compatible Routes for an unselected Client Binding.
The order SHALL be declared, stable, and visible. A Route absent from that
recommendation remains available to explicit client-scoped selection but SHALL
NOT enter automatic onboarding selection. An explicit selection SHALL outrank
all recommendations until its owner changes it. Export SHALL publish the
declared recommendations unchanged and SHALL NOT derive one from a Client
Binding.

#### Scenario: A preferred Account is unavailable before activation

- **WHEN** an unselected client has several recommended compatible Routes and
  only a lower-ranked Account Token is available
- **THEN** setup or sync MAY select that usable Route without requiring the
  higher-ranked Account
- **AND** SHALL report the choice and preserve all unrelated client bindings.

#### Scenario: A manual-only provider is configured

- **WHEN** an Account and Route exist outside every recommendation for a client
- **THEN** AIGW SHALL retain them for explicit use
- **AND** SHALL NOT select them merely because another provider is unavailable.

#### Scenario: A selected provider fails during a request

- **WHEN** a client or an external endpoint reports a transient request failure
- **THEN** AIGW SHALL NOT claim to have observed or retried that request
- **AND** SHALL NOT silently rewrite the selected Route or replay the call
- **AND** MAY present a tested compatible alternative for explicit switching.

#### Scenario: An operator exports a team manifest

- **WHEN** the operator's Client Bindings select a recommended alternative or a
  manual-only Route and the operator runs `aigw config export`
- **THEN** the manifest SHALL carry the declared recommendations unchanged
- **AND** SHALL NOT contain a recommendation derived from those bindings.

### Requirement: Ambiguous client protocols require an explicit choice

When a Route and Client Binding admit more than one endpoint protocol, AIGW SHALL
preserve an existing explicit protocol or require the operator to choose one.
It SHALL NOT infer a protocol from the Model name or silently prefer an
endpoint. Direct setup and Route selection SHALL expose that choice through
the same protocol names used by the team manifest.

#### Scenario: Hermes selects a multi-protocol Route

- **WHEN** Hermes selects a Route with multiple compatible protocols and no
  prior or recommended protocol applies
- **THEN** non-interactive selection names the required `--protocol` choice
- **AND** interactive selection offers each compatible protocol
- **AND** a failed or incompatible choice leaves every Client Binding unchanged.

#### Scenario: A selected protocol requires a new Account Token

- **WHEN** direct setup or interactive Route selection obtains a missing Token
  for a client with multiple compatible protocols
- **THEN** validation probes only the explicitly selected protocol and endpoint
- **AND** a failed probe does not store the Token or change the Client Binding.

#### Scenario: A non-interactive refusal names its retry

- **WHEN** non-interactive Route selection or direct setup refuses because more
  than one protocol is compatible
- **THEN** the recommended action, including JSON `next_action`, names the same
  operation with `--protocol` and the compatible protocol names
- **AND** Route selection names its exact retry,
  `aigw use --for <client> --protocol <a|b> <route>`.

### Requirement: Forwarding selection belongs to one Client Binding

A Client Binding MAY explicitly select a forwarding endpoint independently of
its Account upstream endpoint. The native selection, resolver, credential reader,
guarded projection, dry-run and rollback SHALL derive the same effective
destination from that binding. Selection SHALL preserve the Account credential,
Route, provider identity, model and reasoning choices, unrelated clients and
unmanaged content. AIGW SHALL NOT manage the external service or carry requests.

#### Scenario: One client selects an installed forwarding endpoint

- **WHEN** an enabled client explicitly selects a forwarding endpoint
- **THEN** credential-free preview SHALL identify only that client's changes
- **AND** native selection SHALL preserve the upstream Account and other clients
- **AND** the next sync SHALL retain the selected forwarding endpoint.

#### Scenario: A cached predecessor still requests its direct credential

- **WHEN** a retained client uses its original direct credential reference after
  forwarding selection
- **THEN** the native reader SHALL resolve that exact admitted Account reference
- **AND** SHALL NOT copy a Token, require a new provider identity or rewrite
  conversation history.

#### Scenario: Forwarding selection is accepted or rolled back

- **WHEN** the selected forwarding service is installed and exercised by the
  actual native client
- **THEN** fresh and cold-resumed child task, message and result loops SHALL pass
- **AND** rollback SHALL restore the exact prior client semantics and usable
  credential reference
- **AND** acceptance SHALL verify cleanup of the exact owned test resources.

### Requirement: Hermes selection says existing sessions keep their route

When `aigw use` changes, or previews a change to, the projection of a client
whose existing sessions keep the route they were created with, such as Hermes,
human output SHALL say that existing sessions keep that route and new sessions
use the selection. AIGW SHALL NOT change a native session's route.

#### Scenario: An operator selects another Route for Hermes

- **WHEN** `aigw use --for hermes` changes the Hermes projection, or its preview
  would
- **THEN** human output SHALL state that existing sessions keep their route and
  new sessions use the selection
- **AND** JSON output and Hermes sessions SHALL stay unchanged.

#### Scenario: The projection stays the same

- **WHEN** a selection leaves the Hermes projection unchanged
- **THEN** human output SHALL NOT add the statement.

### Requirement: Retry policy belongs to one client connection

A Client Binding SHALL own optional native request and stream retry budgets for
its selected connection. Undeclared budgets SHALL preserve client defaults.
Declaration, preview, resolution, projection, persistence and recovery SHALL
agree. AIGW SHALL NOT infer policy from provider names or URLs, execute retries,
or promise exactly-once inference.

#### Scenario: An operator delegates recovery to the endpoint

- **WHEN** Codex selects a connection with both retry budgets explicitly zero
- **THEN** its native provider table SHALL contain both zero values
- **AND** public preview and subsequent sync SHALL preserve the declaration,
  Account, provider identity, model, reasoning settings and unrelated clients.

#### Scenario: Native defaults remain undeclared

- **WHEN** a connection has no retry policy, or the operator explicitly selects
  native retries
- **THEN** AIGW SHALL omit the corresponding native provider fields
- **AND** SHALL NOT substitute a guessed default.

#### Scenario: A connection changes while policy is selected

- **WHEN** a selected upstream identity or effective destination changes
- **THEN** selection SHALL require explicit retry intent before any write
- **AND** a model-only selection SHALL retain the same connection policy
- **AND** an explicit verification override for a different connection SHALL NOT
  inherit that policy.

#### Scenario: The client cannot project the declared policy

- **WHEN** the selected client has no admitted native retry-budget capability
- **THEN** AIGW SHALL refuse before credential access, network access or writes
- **AND** SHALL NOT replace the capability with a helper or wrapper.

### Requirement: Retry policy preserves guarded recovery

Retry-policy changes SHALL use the existing guarded selection transaction and
native recovery. External edits SHALL remain semantic client-configuration
conflicts. Program rollback SHALL test the retained reader's actual ability to
read the declaration, refuse without mutation when incompatible, and SHALL NOT
silently lose policy or require manual field deletion.

#### Scenario: A managed budget was changed outside AIGW

- **WHEN** a native budget differs from the recorded owned projection
- **THEN** sync and selection previews SHALL identify a Codex configuration
  conflict without presenting it as a TOML syntax error
- **AND** SHALL preserve the external edit and owned state.

#### Scenario: A policy transaction fails after persistence

- **WHEN** native projection fails after the policy configuration was written
- **THEN** compensation SHALL restore exact unchanged owned preimages
- **AND** SHALL preserve newer external edits and report incomplete compensation.

#### Scenario: The retained program cannot read policy

- **WHEN** program rollback's isolated configuration probe refuses the declaration
- **THEN** the active and retained programs, configuration and credentials SHALL
  remain unchanged
- **AND** recovery SHALL name a compatible program or native configuration
  rollback, not manual policy deletion or an arbitrary version threshold.
