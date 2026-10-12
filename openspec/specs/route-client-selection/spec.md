# route-client-selection Specification

## Purpose

Define deterministic client selection from explicit operator intent, reusable
Routes, and independent Client Bindings without inference from names or provider
details.

## Requirements

### Requirement: Route selection is explicitly client-scoped

A Route SHALL identify one Account, one canonical Model, its exact upstream
identifier, and admitted protocol interfaces independently of client state.
A Client Binding SHALL select one compatible Route for one client.
Non-interactive selection SHALL require both identities; interactive selection
MAY prompt only for admitted choices. An explicit Codex forwarding destination
SHALL remain separate from the selected Account upstream.

#### Scenario: Select a Route for one client

- **WHEN** the operator runs `aigw use --for <client> <route>`
- **THEN** AIGW SHALL update only that client's binding and owned projection
- **AND** SHALL preserve every other client's selection, files, and credentials.

#### Scenario: One Route serves several clients

- **GIVEN** one Route exposes interfaces admitted by several clients
- **WHEN** the operator selects it independently for those clients
- **THEN** AIGW SHALL retain one Route and one Client Binding per client
- **AND** SHALL NOT duplicate the Route or introduce a global selection.

#### Scenario: Preview a Codex forwarding selection

- **WHEN** the operator supplies `--forwarding-endpoint`, `--dry-run`, and
  `--json` to an explicit Codex Route selection
- **THEN** AIGW SHALL report the proposed destination and Account upstream
- **AND** SHALL plan only the selected client's exact targets and actions,
  rejecting owned-target conflicts before rendering a successful preview
- **AND** SHALL read no credential value, issue no network request, and change
  no configuration, projection, backup, or checkpoint.

#### Scenario: Select and remove a forwarding destination

- **WHEN** the operator selects an explicit Codex forwarding destination or
  uses `--direct` to restore its selected Account endpoint
- **THEN** only the Codex destination SHALL change
- **AND** Account, provider, credential-reader identity, other clients, models,
  effort, and conversation state SHALL remain unchanged
- **AND** invalid or incompatible destinations and conflicting flags SHALL
  fail before credential access or writes.

#### Scenario: Preserve forwarding on an unchanged upstream

- **WHEN** ordinary Route selection retains the same Codex Account and protocol
- **THEN** AIGW SHALL preserve its explicit forwarding destination
- **AND** explicit Route selection changing the upstream identity SHALL withdraw
  the old destination.

#### Scenario: A direct Account or protocol edit invalidates forwarding

- **WHEN** an Account or protocol edit differs from the recorded forwarding
  upstream identity
- **THEN** AIGW SHALL refuse validation and persistence without changing owned files
- **AND** SHALL require restoring direct mode before editing that upstream
- **AND** SHALL NOT silently rebind the old forwarding destination.

### Requirement: Connectivity and verification use explicit Route semantics

`aigw test` SHALL test selected client endpoints without changing bindings.
`aigw verify` SHALL perform a real bounded client request. An explicit `--route`
override SHALL require exactly one explicit client and SHALL NOT mutate its
stored binding.

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
