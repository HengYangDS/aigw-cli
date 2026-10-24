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

#### Scenario: A rooted Windows catalog reference keeps its drive

- **WHEN** on Windows the operator's own `model_catalog_json` is rooted without
  a drive, such as `\catalogs\models.json`
- **THEN** the verification projection SHALL name that path on the drive of the
  configuration that names it
- **AND** a reference that names a drive without a root, such as
  `D:models.json`, SHALL stay unchanged.
