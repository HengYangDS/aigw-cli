## MODIFIED Requirements

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
